package dial

import (
	"fmt"
	"net"
	"net/netip"

	E "github.com/lzpls/enimul/internal/errors"
	F "github.com/lzpls/enimul/internal/fmt"
)

type networkInterface struct {
	index   int
	name    string
	gateway string
	ipv4    net.IP
	ipv6    net.IP
}

type networkInterfaces []networkInterface

var errNoInterface = E.New("no interface detected")

func getFilteredInterfaces() (networkInterfaces, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, E.WithStr("list network interfaces", err)
	}

	interfaces := make([]networkInterface, 0, len(ifaces))
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			F.Printf("Get addresses for interface %s: %v", iface.Name, err)
			continue
		}

		var ipv4, ipv6 net.IP
		for _, addr := range addrs {
			if ipv4 != nil && ipv6 != nil {
				break
			}
			ipNet, isIPNet := addr.(*net.IPNet)
			if !isIPNet {
				continue
			}
			ip := ipNet.IP
			if ip.IsLinkLocalUnicast() {
				continue
			}
			isIPv4 := ip.To4() != nil
			if ipv4 == nil && isIPv4 {
				ipv4 = ip
			} else if ipv6 == nil && !isIPv4 && ip.To16() != nil {
				ipv6 = ip
			}
		}

		if ipv4 == nil && ipv6 == nil {
			continue
		}

		interfaces = append(interfaces, networkInterface{
			index:   iface.Index,
			name:    iface.Name,
			gateway: getGatewayForInterface(iface.Index),
			ipv4:    ipv4,
			ipv6:    ipv6,
		})
	}
	if len(interfaces) == 0 {
		return nil, errNoInterface
	}
	return interfaces, nil
}

func (ifaces networkInterfaces) find(name string) (*networkInterface, bool) {
	for i := range ifaces {
		if ifaces[i].name == name {
			return &ifaces[i], true
		}
	}
	return nil, false
}

func (ifaces networkInterfaces) autoSelect(preferredPrefix netip.Prefix) (*networkInterface, bool) {
	if preferredPrefix.IsValid() {
		for _, iface := range ifaces {
			if iface.ipv4 != nil {
				addr, ok := netip.AddrFromSlice(iface.ipv4)
				if ok && preferredPrefix.Contains(addr.Unmap()) {
					return &iface, true
				}
			}
		}
	}
	for _, iface := range ifaces {
		if iface.gateway != "" && iface.ipv4 != nil && iface.ipv4.IsPrivate() {
			return &iface, true
		}
	}
	for _, iface := range ifaces {
		if iface.gateway != "" && iface.ipv4 != nil {
			return &iface, true
		}
	}
	return nil, false
}

func (ifaces networkInterfaces) manualSelect() *networkInterface {
	fmt.Println("Avalable Interfaces:")
	for i, iface := range ifaces {
		msg := F.Concat("[", i, "] ", iface.name)
		if iface.gateway != "" {
			msg += " via " + iface.gateway
		}
		msg += ":"
		if iface.ipv4 != nil {
			msg += " ipv4=" + iface.ipv4.String()
		}
		if iface.ipv6 != nil {
			msg += " ipv6=" + iface.ipv6.String()
		}
		fmt.Println(msg)
	}

	for {
		fmt.Print("Select index: ")
		var i int
		_, err := fmt.Scanln(&i)
		if err != nil {
			fmt.Println(err)
		}
		if i < 0 || i >= len(ifaces) {
			fmt.Println("Invalid index")
			continue
		}
		fmt.Println()
		return &ifaces[i]
	}
}
