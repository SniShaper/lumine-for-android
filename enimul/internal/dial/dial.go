package dial

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	E "github.com/lzpls/enimul/internal/errors"
)

// TCP-only.
var (
	globalIPv4Dialer atomic.Pointer[net.Dialer]
	globalIPv6Dialer atomic.Pointer[net.Dialer]
)

// Note: the returned *net.Dialer is a shallow copy – you may change Timeout, Control,
// etc., but do NOT modify Resolver (they share the global instances).
func NewDialer(isIPv6 bool) *net.Dialer {
	var d net.Dialer
	if isIPv6 {
		d = *globalIPv6Dialer.Load()
	} else {
		d = *globalIPv4Dialer.Load()
	}
	return &d
}

func DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	var dialer *net.Dialer
	if address[0] == '[' {
		dialer = globalIPv6Dialer.Load()
	} else {
		dialer = globalIPv4Dialer.Load()
	}
	return dialer.DialContext(ctx, network, address)
}

func DialTimeout(ctx context.Context, network, address string, timeout time.Duration) (net.Conn, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return DialContext(timeoutCtx, network, address)
}

func DialTCPTimeout(address string, timeout time.Duration) (net.Conn, error) {
	return DialTimeout(context.Background(), "tcp", address, timeout)
}

type monitor = func() (net.IP, net.IP, string, error)

// laddrMonitorState 跟踪运行中的本地地址监测 goroutine，便于重载/关闭时停止。
var (
	monitorMu     sync.Mutex
	monitorStopCh chan struct{}
)

func laddrMonitor(interval time.Duration, fn monitor, stopCh chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			ipv4, ipv6, zone, err := fn()
			if err != nil {
				logger.Error("Failed to update local address: ", err)
				continue
			}
			msg := []any{"Local address updated:"}
			if ipv4 != nil {
				globalIPv4Dialer.Store(&net.Dialer{LocalAddr: &net.TCPAddr{IP: ipv4}})
				msg = append(msg, " ipv4=", ipv4)
			}
			if ipv6 != nil {
				globalIPv6Dialer.Store(&net.Dialer{LocalAddr: &net.TCPAddr{IP: ipv6, Zone: zone}})
				msg = append(msg, " ipv6=", ipv6)
			}
			if zone != "" {
				msg = append(msg, " zone=\"", zone, "\"")
			}
			logger.Info(msg...)
		case <-stopCh:
			return
		}
	}
}

// stopLocalAddrMonitor 停止当前运行中的本地地址监测 goroutine（若有）。
func stopLocalAddrMonitor() {
	monitorMu.Lock()
	defer monitorMu.Unlock()
	if monitorStopCh != nil {
		close(monitorStopCh)
		monitorStopCh = nil
	}
}

// StopLocalAddrMonitor 停止本地地址周期监测 goroutine（引擎关闭时调用）。
func StopLocalAddrMonitor() {
	stopLocalAddrMonitor()
}

var errNoInterfaceWithGateway = E.New("no interface with gateway detected")

func SetLocalAddr(o BindingOption) error {
	var (
		ipv4, ipv6 net.IP
		zone       string
		monitor    monitor
	)
	switch o.Method {
	case MethodOff:
	case MethodSelectInterface:
		interfaces, err := getFilteredInterfaces()
		if err != nil {
			return err
		}
		var selected *networkInterface
		var ok bool
		if o.Zone != "" {
			selected, ok = interfaces.find(o.Zone)
			if !ok {
				return E.New("interface not found: " + o.Zone)
			}
			zone = o.Zone
		} else if o.ManualSelect {
			selected, err := interfaces.manualSelect()
			if err != nil {
				return err
			}
			zone = selected.name
		} else {
			selected, ok = interfaces.autoSelect(o.PreferredPrefix)
			if !ok {
				fmt.Fprintln(os.Stderr, "No interface with gateway detected")
				selected, err = interfaces.manualSelect()
				if err != nil {
					return err
				}
				zone = selected.name
			}
		}
		ipv4, ipv6 = selected.ipv4, selected.ipv6
		if o.UpdateInterval > 0 {
			monitor = func() (net.IP, net.IP, string, error) {
				interfaces, err := getFilteredInterfaces()
				if err != nil {
					return nil, nil, "", err
				}
				var selected *networkInterface
				var ok bool
				if zone == "" {
					selected, ok = interfaces.autoSelect(o.PreferredPrefix)
					if !ok {
						return nil, nil, "", errNoInterfaceWithGateway
					}
				} else {
					selected, ok = interfaces.find(zone)
					if !ok {
						return nil, nil, "", E.New("interface not found: " + o.Zone)
					}
				}
				return selected.ipv4, selected.ipv6, selected.name, nil
			}
		}
	case MethodDialDetect:
		network := "udp"
		if o.DialTCP {
			network = "tcp"
		}
		var err error
		if o.DialIPv4Target != "" {
			ipv4, _, err = detectByDial(network, o.DialIPv4Target, o.DialTimeout)
			if err != nil {
				return err
			}
		}
		if o.DialIPv6Target != "" {
			ipv6, zone, err = detectByDial(network, o.DialIPv6Target, o.DialTimeout)
			if err != nil {
				return err
			}
		}
		if o.UpdateInterval > 0 {
			monitor = func() (ipv4, ipv6 net.IP, zone string, err error) {
				var err1, err2 error
				if o.DialIPv4Target != "" {
					ipv4, _, err1 = detectByDial(network, o.DialIPv4Target, o.DialTimeout)
				}
				if o.DialIPv6Target != "" {
					ipv6, zone, err2 = detectByDial(network, o.DialIPv6Target, o.DialTimeout)
				}
				err = E.Join(err1, err2)
				return
			}
		}
	case MethodCustom:
		ipv4, ipv6, zone = o.CustomIPv4, o.CustomIPv6, o.CustomZone
	}
	ipv4Dialer, ipv6Dialer := new(net.Dialer), new(net.Dialer)
	if ipv4 != nil {
		ipv4Dialer.LocalAddr = &net.TCPAddr{IP: ipv4}
	}
	globalIPv4Dialer.Store(ipv4Dialer)
	if ipv6 != nil {
		ipv6Dialer.LocalAddr = &net.TCPAddr{IP: ipv6, Zone: zone}
	}
	globalIPv6Dialer.Store(ipv6Dialer)
	// 先停旧 monitor 再按需启动新实例，避免每次配置加载叠加 goroutine。
	stopLocalAddrMonitor()
	if monitor != nil {
		monitorMu.Lock()
		monitorStopCh = make(chan struct{})
		stopCh := monitorStopCh
		monitorMu.Unlock()
		go laddrMonitor(o.UpdateInterval, monitor, stopCh)
	}
	return nil
}
