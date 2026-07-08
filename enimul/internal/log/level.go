package log

import (
	"encoding/json"
	"errors"
	"fmt"
)

type Level int8

const (
	LevelTrace Level = iota - 2
	LevelDebug
	LevelInfo // default
	LevelWarn
	LevelError
	Disabled // disables the logger
)

func ParseLevel(s string) (Level, error) {
	switch s {
	case "TRACE", "trace":
		return LevelTrace, nil
	case "DEBUG", "debug":
		return LevelDebug, nil
	case "INFO", "info":
		return LevelInfo, nil
	case "WARN", "warn", "WARNING", "warning":
		return LevelWarn, nil
	case "ERROR", "error":
		return LevelError, nil
	case "NONE", "none":
		return Disabled, nil
	}
	return 0, errors.New("unknown log level: " + s)
}

func (lvl *Level) UnmarshalJSON(raw []byte) error {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return err
	}
	l, err := ParseLevel(s)
	if err != nil {
		return err
	}
	*lvl = l
	return nil
}

func appendLevel(b []byte, lvl Level) []byte {
	switch lvl {
	case LevelTrace:
		return append(b, "TRACE "...)
	case LevelDebug:
		return append(b, "DEBUG "...)
	case LevelInfo:
		return append(b, "INFO  "...)
	case LevelWarn:
		return append(b, "WARN  "...)
	case LevelError:
		return append(b, "ERROR "...)
	case Disabled:
		panic("appendLevel: unexpected Disabled log level")
	}
	panic(fmt.Sprintf("appendLevel: unknown log level: %d", lvl))
}
