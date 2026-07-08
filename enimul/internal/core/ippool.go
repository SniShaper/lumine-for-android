package core

import (
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/netip"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lzpls/enimul/internal/dial"
	E "github.com/lzpls/enimul/internal/errors"
	F "github.com/lzpls/enimul/internal/fmt"
	"github.com/lzpls/enimul/internal/log"
)

var ipPools map[string]*IPPool

const (
	defaultTimeout        = 1 * time.Second
	defaultUpdateInterval = 30 * time.Minute
	defaultMaxConcurrency = 100
	defaultTopIPCount     = 3
	defaultAttempts       = 4
	maxIPPoolSize         = 1000
	weightScaleFactor     = 10000.0
	minWeight             = 1
	maxWeight             = 10000
)

type IPPool struct {
	logger log.Logger

	waitScanOnStartUp bool
	ips               []string
	fallbackIP        string
	port              uint16
	topIPCount        uint8
	attempts          uint8
	timeout           time.Duration
	updateInterval    time.Duration

	mu          sync.RWMutex
	bestIndexes []int
	bestWeights []int
	totalWeight int
	curValidIPs uint32

	scanMu  sync.Mutex
	sem     chan struct{}
	counter uint32
}

func (p *IPPool) UnmarshalJSON(b []byte) error {
	var tmp struct {
		WaitScanOnStartUp bool     `json:"wait_scan_on_startup"`
		FallbackIP        string   `json:"fallback_ip"`
		IPs               []string `json:"ips"`
		Port              int      `json:"port"`
		TopIPCount        int      `json:"top_ip_count"`
		MaxConcurrency    int      `json:"max_concurrency"`
		Timeout           string   `json:"timeout"`
		UpdateInterval    string   `json:"update_interval"`
		Attempts          int      `json:"attempts"`
	}
	if err := json.Unmarshal(b, &tmp); err != nil {
		return err
	}

	ips, err := parseIPList(tmp.IPs)
	if err != nil {
		return err
	}
	if len(ips) == 0 {
		return E.New("no valid IPs after parsing")
	}
	if len(ips) > maxIPPoolSize {
		return fmt.Errorf("IP pool exceeds maximum size (%d): %d", maxIPPoolSize, len(ips))
	}
	if len(ips) < tmp.TopIPCount {
		return fmt.Errorf("IP count (%d) less than top_ip_count (%d)", len(ips), tmp.TopIPCount)
	}

	if tmp.Port <= 0 || tmp.Port > 65535 {
		return fmt.Errorf("invalid port: %d", tmp.Port)
	}

	concurrency := tmp.MaxConcurrency
	if concurrency == 0 {
		concurrency = defaultMaxConcurrency
	} else if concurrency < 1 {
		return fmt.Errorf("invalid max_concurrency: %d", concurrency)
	}

	topCount := tmp.TopIPCount
	if topCount == 0 {
		topCount = defaultTopIPCount
	} else if topCount <= 0 || topCount > 255 || topCount > len(ips) {
		return fmt.Errorf("invalid top_ip_count: %d", topCount)
	}

	attempts := tmp.Attempts
	if attempts == 0 {
		attempts = defaultAttempts
	} else if attempts <= 0 || attempts > 255 {
		return fmt.Errorf("invalid attempts: %d", attempts)
	}

	timeout := defaultTimeout
	if tmp.Timeout != "" {
		timeout, err = time.ParseDuration(tmp.Timeout)
		if err != nil || timeout <= 0 {
			return fmt.Errorf("invalid timeout: %s", tmp.Timeout)
		}
	}

	updateInterval := defaultUpdateInterval
	if tmp.UpdateInterval != "" {
		updateInterval, err = time.ParseDuration(tmp.UpdateInterval)
		if err != nil || updateInterval <= 0 {
			return fmt.Errorf("invalid update_interval: %s", tmp.UpdateInterval)
		}
	}

	p.waitScanOnStartUp = tmp.WaitScanOnStartUp
	p.ips = ips
	p.fallbackIP = tmp.FallbackIP
	p.port = uint16(tmp.Port)
	p.topIPCount = uint8(topCount)
	p.attempts = uint8(attempts)
	p.timeout = timeout
	p.updateInterval = updateInterval
	p.sem = make(chan struct{}, concurrency)
	p.bestIndexes = make([]int, topCount)
	p.bestWeights = make([]int, topCount)
	return nil
}

func parseIPList(sources []string) ([]string, error) {
	ips := make([]string, 0, len(sources)*10)
	for _, pattern := range sources {
		for _, s := range expandPattern(pattern) {
			if len(ips) >= maxIPPoolSize {
				return nil, fmt.Errorf("IP pool exceeds maximum size (%d) during parsing", maxIPPoolSize)
			}
			if addr, err := netip.ParseAddr(s); err == nil && addr.IsValid() {
				ips = append(ips, s)
				continue
			}
			if prefix, err := netip.ParsePrefix(s); err == nil {
				addr := prefix.Addr()
				for prefix.Contains(addr) {
					if len(ips) >= maxIPPoolSize {
						return nil, fmt.Errorf("CIDR %s exceeds max pool size (%d)", s, maxIPPoolSize)
					}
					ips = append(ips, addr.String())
					next := addr.Next()
					if !next.IsValid() || !prefix.Contains(next) {
						break
					}
					addr = next
				}
				continue
			}
			addrs, err := net.LookupIP(s)
			if err != nil {
				return nil, fmt.Errorf("DNS lookup failed for %s: %w", s, err)
			}
			for _, ip := range addrs {
				if addr, ok := netip.AddrFromSlice(ip); ok && addr.IsValid() {
					if len(ips) >= maxIPPoolSize {
						return nil, fmt.Errorf("DNS resolution for %s exceeds max pool size", s)
					}
					ips = append(ips, addr.String())
				}
			}
		}
	}
	return ips, nil
}

func (p *IPPool) Init(logger log.Logger) {
	p.logger = logger
	if p.waitScanOnStartUp {
		p.scan()
		go p.monitor()
	} else {
		go func() {
			p.scan()
			p.monitor()
		}()
	}
}

type ipResult struct {
	ipIndex int
	latency time.Duration
	loss    float64
}

func (p *IPPool) scan() {
	p.scanMu.Lock()
	defer p.scanMu.Unlock()

	results := make(chan ipResult, len(p.ips))
	var wg sync.WaitGroup

	p.logger.Info("Testing...")

	for i := range p.ips {
		wg.Go(func() {
			p.sem <- struct{}{}
			latency, loss := p.testIP(i)
			<-p.sem
			results <- ipResult{i, latency, loss}
		})
	}

	wg.Wait()
	close(results)

	validResults := make([]ipResult, 0, len(p.ips))
	for res := range results {
		if res.loss < 1.0 {
			validResults = append(validResults, res)
		}
	}

	p.updateBest(validResults)
}

func (p *IPPool) testIP(index int) (time.Duration, float64) {
	var (
		successCount int64
		totalLatency time.Duration
	)
	portStr := F.Uint(p.port)
	addrStr := net.JoinHostPort(p.ips[index], portStr)

	for range p.attempts {
		start := time.Now()
		conn, err := dial.DialTCPTimeout(addrStr, p.timeout)
		if err != nil {
			continue
		}
		totalLatency += time.Since(start)
		conn.Close()
		successCount++
	}

	lossRate := 1.0 - float64(successCount)/float64(p.attempts)
	if successCount == 0 {
		return time.Duration(math.MaxInt64), lossRate
	}
	latency := totalLatency / time.Duration(successCount)
	p.logger.Debug("ip=", p.ips[index], " latency=", latency, " loss=", fmt.Sprintf("%.2f%%", lossRate*100))
	return latency, lossRate
}

func (p *IPPool) updateBest(results []ipResult) {
	if len(results) == 0 {
		return
	}

	sort.SliceStable(results, func(i, j int) bool {
		if results[i].loss != results[j].loss {
			return results[i].loss < results[j].loss
		}
		return results[i].latency < results[j].latency
	})

	p.mu.Lock()
	defer p.mu.Unlock()

	var validCount, totalWeight int
	for i := 0; i < int(p.topIPCount) && i < len(results); i++ {
		res := results[i]
		p.bestIndexes[i] = res.ipIndex

		latencyMs := float64(res.latency) / float64(time.Millisecond)
		weightVal := weightScaleFactor / (latencyMs*(1.0+res.loss) + 1e-5)
		if weightVal < minWeight {
			weightVal = minWeight
		} else if weightVal > maxWeight {
			weightVal = maxWeight
		}
		weight := int(weightVal)
		p.bestWeights[i] = weight
		totalWeight += weight
		validCount++
	}

	for i := validCount; i < int(p.topIPCount); i++ {
		p.bestWeights[i] = 0
	}

	var builder strings.Builder
	builder.Grow(len("Current best IPs: ") + validCount*17)
	builder.WriteString("Current best IPs: ")
	for _, index := range p.bestIndexes[:validCount] {
		builder.WriteString(p.ips[index])
		builder.WriteByte(' ')
	}
	p.logger.Info(builder.String())

	p.curValidIPs = uint32(validCount)
	p.totalWeight = totalWeight
}

func (p *IPPool) monitor() {
	for range time.Tick(p.updateInterval) {
		p.scan()
	}
}

func (p *IPPool) Get() string {
	p.mu.RLock()
	validCount := p.curValidIPs
	if validCount == 0 {
		p.mu.RUnlock()
		return p.fallbackIP
	}
	indexes := append([]int(nil), p.bestIndexes[:validCount]...)
	weights := append([]int(nil), p.bestWeights[:validCount]...)
	total := p.totalWeight
	p.mu.RUnlock()

	current := atomic.AddUint32(&p.counter, 1) - 1
	target := int(current) % total
	acc := 0
	for i := range validCount {
		acc += weights[i]
		if acc > target {
			return p.ips[indexes[i]]
		}
	}
	return p.fallbackIP
}

func getFromIPPool(tag string) (ipStr string, err error) {
	if len(ipPools) == 0 {
		return "", E.New("no ip pools")
	}
	ipPool, exists := ipPools[tag]
	if !exists {
		return "", E.New("ip pool " + tag + " does not exist")
	}
	ip := ipPool.Get()
	if ip == "" {
		return "", E.New("cannot get ip from " + tag)
	}
	return ip, nil
}
