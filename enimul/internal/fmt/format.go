package format

import (
	"fmt"
	"os"
	"strconv"
	"unsafe"
)

const hexDigits = "0123456789abcdef"

func Byte(b byte) string {
	var buf [2]byte
	buf[0] = hexDigits[b>>4]
	buf[1] = hexDigits[b&0xf]
	return string(buf[:])
}

func ConnIDToHex5(prefix string, id uint32) string {
	prefixLen := len(prefix)
	buf := make([]byte, prefixLen+1+5+1)
	copy(buf, prefix)
	buf[prefixLen], buf[len(buf)-1] = '[', ']'
	for i := range 5 {
		buf[prefixLen+i+1] = hexDigits[(id>>uint(4*(4-i)))&0xf]
	}
	return string(buf)
}

func Append(b []byte, args ...any) []byte {
	for _, arg := range args {
		switch a := arg.(type) {
		case fmt.Stringer:
			b = append(b, a.String()...)
		case error:
			b = append(b, a.Error()...)
		case string:
			b = append(b, a...)
		case int:
			b = strconv.AppendInt(b, int64(a), 10)
		case int8:
			b = strconv.AppendInt(b, int64(a), 10)
		case int16:
			b = strconv.AppendInt(b, int64(a), 10)
		case int32:
			b = strconv.AppendInt(b, int64(a), 10)
		case int64:
			b = strconv.AppendInt(b, a, 10)
		case uint:
			b = strconv.AppendUint(b, uint64(a), 10)
		case uint8:
			b = strconv.AppendUint(b, uint64(a), 10)
		case uint16:
			b = strconv.AppendUint(b, uint64(a), 10)
		case uint32:
			b = strconv.AppendUint(b, uint64(a), 10)
		case uint64:
			b = strconv.AppendUint(b, a, 10)
		default:
			b = fmt.Append(b, a)
		}
	}
	return b
}

func Concat(args ...any) string {
	buf := Append(make([]byte, 0, 32), args...)
	return unsafe.String(unsafe.SliceData(buf), len(buf))
}

func Int[T int | int8 | int16 | int32 | int64](v T) string {
	return strconv.FormatInt(int64(v), 10)
}

func Uint[T uint | uint8 | uint16 | uint32 | uint64](v T) string {
	return strconv.FormatUint(uint64(v), 10)
}

func Println(args ...any) { fmt.Fprintln(os.Stderr, args...) }

func Printf(msg string, args ...any) { fmt.Fprintf(os.Stderr, msg, args...) }
