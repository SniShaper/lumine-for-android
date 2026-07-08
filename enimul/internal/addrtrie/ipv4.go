package addrtrie

import (
	"encoding/binary"
	"net"
	"net/netip"
)

func getBit32(v uint32, i int) uint32 {
	return (v >> (31 - i)) & 1
}

func parseIPv4OrCIDR(s string) (ip uint32, bitLen int, err error) {
	prefix, err := netip.ParsePrefix(s)
	if err == nil {
		addr := prefix.Addr()
		if !addr.Is4() {
			return 0, 0, net.InvalidAddrError("non-IPv4 mask")
		}

		bitLen = prefix.Bits()
		if bitLen < 0 || bitLen > 32 {
			return 0, 0, net.InvalidAddrError("non-canonical mask")
		}

		b := addr.As4()
		ip = binary.BigEndian.Uint32(b[:])
		return ip, bitLen, nil
	}

	addr, err := netip.ParseAddr(s)
	if err == nil {
		addr = addr.Unmap()
		if !addr.Is4() {
			return 0, 0, net.InvalidAddrError("invalid IPv4 address")
		}
		b := addr.As4()
		ip = binary.BigEndian.Uint32(b[:])
		return ip, 32, nil
	}

	return 0, 0, net.InvalidAddrError("invalid IPv4 address or CIDR")
}

type bitNode[T any] struct {
	children    [2]*bitNode[T]
	value       T
	valueExists bool
}

type IPv4Trie[T any] struct {
	root *bitNode[T]
}

func NewIPv4Trie[T any]() *IPv4Trie[T] {
	return &IPv4Trie[T]{root: &bitNode[T]{children: [2]*bitNode[T]{}}}
}

func (t *IPv4Trie[T]) Insert(prefix string, value T) error {
	if prefix == "*" {
		t.root.value = value
		t.root.valueExists = true
		return nil
	}
	ip, bitLen, err := parseIPv4OrCIDR(prefix)
	if err != nil {
		return err
	}

	cur := t.root
	for i := range bitLen {
		b := getBit32(ip, i)
		if cur.children[b] == nil {
			cur.children[b] = &bitNode[T]{children: [2]*bitNode[T]{}}
		}
		cur = cur.children[b]
	}
	cur.value = value
	cur.valueExists = true
	return nil
}

func (t *IPv4Trie[T]) Find(ipStr string) (matched T, exists bool) {
	addr, err := netip.ParseAddr(ipStr)
	if err != nil {
		return
	}

	addr = addr.Unmap()
	if !addr.Is4() {
		return
	}

	ipBytes := addr.As4()
	ipUint := binary.BigEndian.Uint32(ipBytes[:])

	cur := t.root
	for i := range 32 {
		if cur.valueExists {
			matched = cur.value
			exists = true
		}
		b := getBit32(ipUint, i)
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
