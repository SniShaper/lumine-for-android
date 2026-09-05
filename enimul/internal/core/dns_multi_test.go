package core

import (
	"errors"
	"testing"

	"github.com/miekg/dns"
)

func stubExchange(respCode int, err error) func(*dns.Msg) (*dns.Msg, error) {
	return func(req *dns.Msg) (*dns.Msg, error) {
		if err != nil {
			return nil, err
		}
		resp := new(dns.Msg)
		resp.SetReply(req)
		resp.Rcode = respCode
		return resp, nil
	}
}

func withDNSExchanges(t *testing.T, exchanges []func(*dns.Msg) (*dns.Msg, error)) {
	t.Helper()
	origExchanges := dnsExchanges
	origSingle := dnsExchange
	origIdx := dnsExchangeIdx.Load()
	t.Cleanup(func() {
		dnsExchanges = origExchanges
		dnsExchange = origSingle
		dnsExchangeIdx.Store(origIdx)
	})
	dnsExchanges = exchanges
}

func TestExchangeMsgFallsBackToSingleExchange(t *testing.T) {
	withDNSExchanges(t, nil)
	dnsExchange = stubExchange(dns.RcodeSuccess, nil)
	if _, err := exchangeMsg(new(dns.Msg)); err != nil {
		t.Fatalf("single-exchange fallback failed: %v", err)
	}
}

func TestExchangeMsgRotatesToHealthyEndpoint(t *testing.T) {
	withDNSExchanges(t, []func(*dns.Msg) (*dns.Msg, error){
		stubExchange(0, errors.New("down")),
		stubExchange(dns.RcodeSuccess, nil),
	})
	msg := new(dns.Msg)
	msg.SetQuestion("example.com.", dns.TypeA)
	resp, err := exchangeMsg(msg)
	if err != nil {
		t.Fatalf("exchangeMsg returned error: %v", err)
	}
	if resp.Rcode != dns.RcodeSuccess {
		t.Fatalf("expected healthy endpoint response, got rcode %d", resp.Rcode)
	}
	if dnsExchangeIdx.Load() != 1 {
		t.Fatalf("sticky index not updated: got %d want 1", dnsExchangeIdx.Load())
	}
	// 健康端点应被粘住：再次调用不经过故障端点
	resp, err = exchangeMsg(msg)
	if err != nil {
		t.Fatalf("second exchangeMsg returned error: %v", err)
	}
	if dnsExchangeIdx.Load() != 1 {
		t.Fatalf("sticky index moved after success: got %d", dnsExchangeIdx.Load())
	}
}
