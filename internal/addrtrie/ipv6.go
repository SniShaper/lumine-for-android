package addrtrie

import (
	"encoding/binary"
	"net"
	"net/netip"
)

type uint128 struct {
	hi uint64
	lo uint64
}

func getBit128(v uint128, i int) uint64 {
	if i < 64 {
		return (v.hi >> (63 - i)) & 1
	}
	return (v.lo >> (63 - (i - 64))) & 1
}

func parseIPv6OrCIDR(s string) (ip uint128, bitLen int, err error) {
	prefix, err := netip.ParsePrefix(s)
	if err == nil {
		addr := prefix.Addr().Unmap()
		if addr.Is4() {
			return uint128{}, 0, net.InvalidAddrError("non-IPv6 mask")
		}

		b := addr.As16()
		ip.hi = binary.BigEndian.Uint64(b[:8])
		ip.lo = binary.BigEndian.Uint64(b[8:16])
		bitLen = prefix.Bits()

		if bitLen < 0 || bitLen > 128 {
			return uint128{}, 0, net.InvalidAddrError("non-canonical mask")
		}
		return ip, bitLen, nil
	}

	addr, err := netip.ParseAddr(s)
	if err == nil {
		addr = addr.Unmap()
		if addr.Is4() {
			return uint128{}, 0, net.InvalidAddrError("invalid IPv6 address")
		}

		b := addr.As16()
		ip.hi = binary.BigEndian.Uint64(b[:8])
		ip.lo = binary.BigEndian.Uint64(b[8:16])
		return ip, 128, nil
	}

	return uint128{}, 0, net.InvalidAddrError("invalid IPv6 address or CIDR")
}

type IPv6Trie[T any] struct {
	root *bitNode[T]
}

func NewIPv6Trie[T any]() *IPv6Trie[T] {
	return &IPv6Trie[T]{root: &bitNode[T]{}}
}

func (t *IPv6Trie[T]) Insert(prefix string, value T) error {
	if prefix == "*" {
		t.root.value = value
		t.root.valueExists = true
		return nil
	}
	ip, bitLen, err := parseIPv6OrCIDR(prefix)
	if err != nil {
		return err
	}

	cur := t.root
	for i := range bitLen {
		b := getBit128(ip, i)
		if cur.children[b] == nil {
			cur.children[b] = &bitNode[T]{}
		}
		cur = cur.children[b]
	}
	cur.value = value
	cur.valueExists = true
	return nil
}

func (t *IPv6Trie[T]) Find(ipStr string) (matched T, exists bool) {
	addr, err := netip.ParseAddr(ipStr)
	if err != nil {
		return
	}

	addr = addr.Unmap()
	if addr.Is4() {
		return
	}

	b := addr.As16()
	ipUint := uint128{
		hi: binary.BigEndian.Uint64(b[:8]),
		lo: binary.BigEndian.Uint64(b[8:16]),
	}

	cur := t.root
	for i := range 128 {
		if cur.valueExists {
			matched = cur.value
			exists = true
		}
		b := getBit128(ipUint, i)
		if cur.children[b] == nil {
			break
		}
		cur = cur.children[b]
	}
	if cur.valueExists {
		matched = cur.value
		exists = true
	}
	if !exists && t.root.valueExists {
		return t.root.value, t.root.valueExists
	}
	return
}
