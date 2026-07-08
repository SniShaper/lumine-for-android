package lumine

import (
	"fmt"
	"strconv"
	"strings"
)

func byteToString(b byte) string {
	return fmt.Sprintf("%x", b)
}

func joinString(values ...any) string {
	var builder strings.Builder
	for _, v := range values {
		switch val := v.(type) {
		case fmt.Stringer:
			builder.WriteString(val.String())
		case string:
			builder.WriteString(val)
		case error:
			builder.WriteString(val.Error())
		case int:
			builder.WriteString(strconv.Itoa(val))
		case int8:
			builder.WriteString(formatInt(val))
		case int16:
			builder.WriteString(formatInt(val))
		case int32:
			builder.WriteString(formatInt(val))
		case int64:
			builder.WriteString(formatInt(val))
		case uint:
			builder.WriteString(formatUint(val))
		case uint8:
			builder.WriteString(formatUint(val))
		case uint16:
			builder.WriteString(formatUint(val))
		case uint32:
			builder.WriteString(formatUint(val))
		case uint64:
			builder.WriteString(formatUint(val))
		default:
			panic("unsupported type")
		}
	}
	return builder.String()
}

func formatInt[T int | int8 | int16 | int32 | int64](v T) string {
	return strconv.FormatInt(int64(v), 10)
}

func formatUint[T uint | uint8 | uint16 | uint32 | uint64](v T) string {
	return strconv.FormatUint(uint64(v), 10)
}
