package lumine

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"syscall"

	log "github.com/moi-si/mylog"
)

func findLastDot(data []byte, sniPos, sniLen int) (offset int, found bool) {
	for i := sniPos + sniLen; i >= sniPos; i-- {
		if data[i] == '.' {
			return i, true
		}
	}
	return sniLen/2 + sniPos, false
}

func parseClientHello(data []byte) (prtVer []byte, sniPos int, sniLen int, hasKeyShare bool, err error) {
	const (
		recordHeaderLen          = 5
		handshakeHeaderLen       = 4
		handshakeTypeClientHello = 0x01
		extTypeSNI               = 0x0000
		extTypeKeyShare          = 0x0033
	)

	prtVer = nil
	sniPos = -1
	sniLen = 0

	recordLen := int(binary.BigEndian.Uint16(data[3:5]))
	if len(data) < recordHeaderLen+recordLen {
		return prtVer, sniPos, sniLen, false, errors.New("record length exceeds data size")
	}
	offset := recordHeaderLen

	if recordLen < handshakeHeaderLen {
		return prtVer, sniPos, sniLen, false, errors.New("handshake message too short")
	}
	if data[offset] != handshakeTypeClientHello {
		return prtVer, sniPos, sniLen, false, fmt.Errorf("not a ClientHello handshake (type=%d)", data[offset])
	}
	handshakeLen := int(uint32(data[offset+1])<<16 | uint32(data[offset+2])<<8 | uint32(data[offset+3]))
	if handshakeLen+handshakeHeaderLen > recordLen {
		return prtVer, sniPos, sniLen, false, errors.New("handshake length exceeds record length")
	}
	offset += handshakeHeaderLen

	if handshakeLen < 2+32+1 {
		return prtVer, sniPos, sniLen, false, errors.New("ClientHello too short for mandatory fields")
	}
	prtVer = data[offset : offset+2]
	offset += 2
	offset += 32
	if offset >= len(data) {
		return prtVer, sniPos, sniLen, false, errors.New("unexpected end after Random")
	}
	sessionIDLen := int(data[offset])
	offset++
	if offset+sessionIDLen > len(data) {
		return prtVer, sniPos, sniLen, false, errors.New("session_id length exceeds data")
	}
	offset += sessionIDLen

	if offset+2 > len(data) {
		return prtVer, sniPos, sniLen, false, errors.New("cannot read cipher_suites length")
	}
	csLen := int(binary.BigEndian.Uint16(data[offset : offset+2]))
	offset += 2
	if offset+csLen > len(data) {
		return prtVer, sniPos, sniLen, false, errors.New("cipher_suites exceed data")
	}
	offset += csLen

	if offset >= len(data) {
		return prtVer, sniPos, sniLen, false, errors.New("cannot read compression_methods length")
	}
	compMethodsLen := int(data[offset])
	offset++
	if offset+compMethodsLen > len(data) {
		return prtVer, sniPos, sniLen, false, errors.New("compression_methods exceed data")
	}
	offset += compMethodsLen

	// Extensions
	if offset+2 > len(data) {
		return prtVer, sniPos, sniLen, false, nil
	}
	extTotalLen := int(binary.BigEndian.Uint16(data[offset : offset+2]))
	offset += 2
	if offset+extTotalLen > len(data) {
		return prtVer, sniPos, sniLen, false, errors.New("extensions length exceeds data")
	}
	extensionsEnd := offset + extTotalLen

	for offset+4 <= extensionsEnd {
		extType := binary.BigEndian.Uint16(data[offset : offset+2])
		extLen := int(binary.BigEndian.Uint16(data[offset+2 : offset+4]))
		extDataStart := offset + 4
		extDataEnd := extDataStart + extLen

		if extDataEnd > extensionsEnd {
			return prtVer, sniPos, sniLen, false, errors.New("extension length exceeds extensions block")
		}

		if extType == extTypeKeyShare {
			hasKeyShare = true
			if sniPos != -1 {
				return prtVer, sniPos, sniLen, hasKeyShare, nil
			}
		}

		if sniPos == -1 && extType == extTypeSNI {
			if extLen < 2 {
				return prtVer, sniPos, sniLen, hasKeyShare, errors.New("malformed SNI extension (too short for list length)")
			}
			listLen := int(binary.BigEndian.Uint16(data[extDataStart : extDataStart+2]))
			if listLen+2 != extLen {
				return prtVer, sniPos, sniLen, hasKeyShare, errors.New("SNI list length field mismatch")
			}
			cursor := extDataStart + 2
			if cursor+3 > extDataEnd {
				return prtVer, sniPos, sniLen, hasKeyShare, errors.New("SNI entry too short")
			}
			nameType := data[cursor]
			if nameType != 0 {
				return prtVer, sniPos, sniLen, hasKeyShare, errors.New("unsupported SNI name type")
			}
			nameLen := int(binary.BigEndian.Uint16(data[cursor+1 : cursor+3]))
			nameStart := cursor + 3
			nameEnd := nameStart + nameLen
			if nameEnd > extDataEnd {
				return prtVer, sniPos, sniLen, hasKeyShare, errors.New("SNI name length exceeds extension")
			}
			sniPos = nameStart
			sniLen = nameLen
			if hasKeyShare {
				return prtVer, sniPos, sniLen, hasKeyShare, nil
			}
		}
		offset = extDataEnd
	}
	return prtVer, sniPos, sniLen, hasKeyShare, nil
}

func expandPattern(s string) []string {
	left := -1
	for i, ch := range s {
		if ch == '(' {
			left = i
			break
		}
	}

	if left == -1 {
		return splitByPipe(s)
	}

	right := -1
	depth := 1
	for i := left + 1; i < len(s); i++ {
		if s[i] == '(' {
			depth++
		} else if s[i] == ')' {
			depth--
			if depth == 0 {
				right = i
				break
			}
		}
	}

	if right == -1 {
		return splitByPipe(s)
	}

	prefix := s[:left]
	inner := s[left+1 : right]
	suffix := s[right+1:]

	parts := splitByPipe(inner)

	suffixResults := expandPattern(suffix)

	result := make([]string, 0, len(parts)*len(suffixResults))
	for _, part := range parts {
		for _, suff := range suffixResults {
			result = append(result, prefix+part+suff)
		}
	}

	return result
}

func splitByPipe(s string) []string {
	if s == "" {
		return []string{""}
	}
	result := []string{}
	curr := ""
	for _, ch := range s {
		if ch == '|' {
			result = append(result, curr)
			curr = ""
		} else {
			curr += string(ch)
		}
	}
	result = append(result, curr)
	return result
}

func isIPv6(ip string) bool {
	return strings.Contains(ip, ":")
}

func transformIP(ipStr string, targetNetStr string) (string, error) {
	ip, err := netip.ParseAddr(ipStr)
	if err != nil {
		return "", errors.New("invalid IP")
	}

	prefix, err := netip.ParsePrefix(targetNetStr)
	if err != nil {
		return "", wrap("invalid target network", err)
	}

	if ip.Is4() != prefix.Addr().Is4() {
		return "", errors.New("IP version mismatch between source IP and target network")
	}

	networkAddr := prefix.Masked().Addr()
	bits := prefix.Bits()

	var newIP netip.Addr
	if ip.Is4() {
		ipBytes := ip.As4()
		netBytes := networkAddr.As4()
		var newBytes [4]byte

		for i := range 4 {
			bitPos := uint8(i * 8)
			if bits >= int(bitPos+8) {
				newBytes[i] = netBytes[i]
			} else if bits <= int(bitPos) {
				newBytes[i] = ipBytes[i]
			} else {
				maskBits := uint8(bits) - bitPos
				mask := uint8(0xFF << (8 - maskBits))
				newBytes[i] = (netBytes[i] & mask) | (ipBytes[i] & ^mask)
			}
		}
		newIP = netip.AddrFrom4(newBytes)
	} else {
		ipBytes := ip.As16()
		netBytes := networkAddr.As16()
		var newBytes [16]byte

		for i := range 16 {
			bitPos := uint8(i * 8)
			if bits >= int(bitPos+8) {
				newBytes[i] = netBytes[i]
			} else if bits <= int(bitPos) {
				newBytes[i] = ipBytes[i]
			} else {
				maskBits := uint8(bits) - bitPos
				mask := uint8(0xFF << (8 - maskBits))
				newBytes[i] = (netBytes[i] & mask) | (ipBytes[i] & ^mask)
			}
		}
		newIP = netip.AddrFrom16(newBytes)
	}

	return newIP.String(), nil
}

func ipRedirect(logger *log.Logger, ip string) (string, *Policy, error) {
	policy, exists := getIPPolicy(ip)
	if !exists {
		return ip, nil, nil
	}
	if policy.MapTo == nil || *policy.MapTo == "" {
		return ip, policy, nil
	}
	var err error
	mapTo := *policy.MapTo
	if strings.HasPrefix(mapTo, tagPrefix) {
		if mapTo, err = getFromIPPool(mapTo[1:]); err != nil {
			return "", nil, err
		}
	} else if strings.LastIndexByte(*policy.MapTo, '/') != -1 {
		mapTo, err = transformIP(ip, *policy.MapTo)
		if err != nil {
			return "", nil, err
		}
	} else if net.ParseIP(mapTo) == nil {
		mapTo, err = resolveBootstrapHost(mapTo)
		if err != nil {
			return "", nil, err
		}
	}
	if logger != nil && ip != mapTo {
		logger.Debug("Redirect:", ip, "->", mapTo)
	} else {
		policy, _ = getIPPolicy(mapTo)
	}
	return mapTo, policy, nil
}

func getTCPRawConn(conn net.Conn) (syscall.RawConn, error) {
	rawConn, err := conn.(*net.TCPConn).SyscallConn()
	if err != nil {
		return nil, wrap("get raw conn", err)
	}
	return rawConn, nil
}

func isUseOfClosedConn(err error) bool {
	return strings.Contains(err.Error(), "use of closed")
}

type wrappedError struct {
	msg   string
	cause error
}

func (e *wrappedError) Error() string {
	return e.msg + ": " + e.cause.Error()
}

func (e *wrappedError) Unwrap() error {
	return e.cause
}

func wrap(msg string, cause error) error {
	return &wrappedError{
		msg:   msg,
		cause: cause,
	}
}

func bytesHasPrefix(b []byte, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if string(b[:len(prefix)]) == prefix {
			return true
		}
	}
	return false
}
