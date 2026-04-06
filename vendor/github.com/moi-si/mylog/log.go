// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// MODIFIED VERSION: This file has been substantially modified from the original
// Go standard library. Modifications are licensed under the same BSD-style license.
// See LICENSE file for full terms.

package mylog

import (
	"fmt"
	"io"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

type Level uint8

const (
	DEBUG Level = iota
	INFO
	ERROR
)

const (
	Ldate = 1 << iota
	Ltime
	Lmicroseconds
	Llongfile
	Lshortfile
	LUTC
	LstdFlags = Ldate | Ltime
)

type Logger struct {
	outMu sync.Mutex
	out   io.Writer

	prefix    atomic.Pointer[string]
	flag      atomic.Int32
	isDiscard atomic.Bool
	minLevel  atomic.Int32
}

func New(out io.Writer, prefix string, flag int, level Level) *Logger {
	l := new(Logger)
	l.SetOutput(out)
	l.SetPrefix(prefix)
	l.SetFlags(flag)
	l.SetLevel(level)
	return l
}

func (l *Logger) SetOutput(w io.Writer) {
	l.outMu.Lock()
	defer l.outMu.Unlock()
	l.out = w
	l.isDiscard.Store(w == io.Discard)
}

func itoa(buf *[]byte, i int, wid int) {
	var b [20]byte
	bp := len(b) - 1
	for i >= 10 || wid > 1 {
		wid--
		q := i / 10
		b[bp] = byte('0' + i - q*10)
		bp--
		i = q
	}
	b[bp] = byte('0' + i)
	*buf = append(*buf, b[bp:]...)
}

func formatHeader(buf *[]byte, t time.Time, prefix string, flag int, levelStr string, file string, line int) {
	*buf = append(*buf, prefix...)
	*buf = append(*buf, levelStr...)

	if flag&(Ldate|Ltime|Lmicroseconds) != 0 {
		if flag&LUTC != 0 {
			t = t.UTC()
		}
		if flag&Ldate != 0 {
			year, month, day := t.Date()
			itoa(buf, year, 4)
			*buf = append(*buf, '/')
			itoa(buf, int(month), 2)
			*buf = append(*buf, '/')
			itoa(buf, day, 2)
			*buf = append(*buf, ' ')
		}
		if flag&(Ltime|Lmicroseconds) != 0 {
			hour, min, sec := t.Clock()
			itoa(buf, hour, 2)
			*buf = append(*buf, ':')
			itoa(buf, min, 2)
			*buf = append(*buf, ':')
			itoa(buf, sec, 2)
			if flag&Lmicroseconds != 0 {
				*buf = append(*buf, '.')
				itoa(buf, t.Nanosecond()/1e3, 6)
			}
			*buf = append(*buf, ' ')
		}
	}

	if flag&(Lshortfile|Llongfile) != 0 {
		if flag&Lshortfile != 0 {
			short := file
			for i := len(file) - 1; i > 0; i-- {
				if file[i] == '/' {
					short = file[i+1:]
					break
				}
			}
			file = short
		}
		*buf = append(*buf, file...)
		*buf = append(*buf, ':')
		itoa(buf, line, -1)
		*buf = append(*buf, ": "...)
	}
}

var bufferPool = sync.Pool{New: func() any { return new([]byte) }}

func getBuffer() *[]byte {
	p := bufferPool.Get().(*[]byte)
	*p = (*p)[:0]
	return p
}

func putBuffer(p *[]byte) {
	if cap(*p) > 64<<10 {
		*p = nil
	}
	bufferPool.Put(p)
}

func (l *Logger) output(level Level, pc uintptr, calldepth int, appendOutput func([]byte) []byte) error {
	if int32(level) < l.minLevel.Load() {
		return nil
	}

	if l.isDiscard.Load() {
		return nil
	}

	now := time.Now()

	prefix := l.Prefix()
	flag := l.Flags()

	var file string
	var line int
	if flag&(Lshortfile|Llongfile) != 0 {
		if pc == 0 {
			var ok bool
			_, file, line, ok = runtime.Caller(calldepth)
			if !ok {
				file = "???"
				line = 0
			}
		} else {
			fs := runtime.CallersFrames([]uintptr{pc})
			f, _ := fs.Next()
			file = f.File
			if file == "" {
				file = "???"
			}
			line = f.Line
		}
	}

	var levelStr string
	switch level {
	case DEBUG:
		levelStr = "[DEBUG] "
	case INFO:
		levelStr = "[INFO]  "
	case ERROR:
		levelStr = "[ERROR] "
	default:
		levelStr = "[?????] "
	}

	buf := getBuffer()
	defer putBuffer(buf)
	formatHeader(buf, now, prefix, flag, levelStr, file, line)
	*buf = appendOutput(*buf)
	if len(*buf) == 0 || (*buf)[len(*buf)-1] != '\n' {
		*buf = append(*buf, '\n')
	}

	l.outMu.Lock()
	defer l.outMu.Unlock()
	_, err := l.out.Write(*buf)
	return err
}

func (l *Logger) Debug(v ...any) {
	l.output(DEBUG, 0, 2, func(b []byte) []byte {
		return fmt.Appendln(b, v...)
	})
}

func (l *Logger) Info(v ...any) {
	l.output(INFO, 0, 2, func(b []byte) []byte {
		return fmt.Appendln(b, v...)
	})
}

func (l *Logger) Error(v ...any) {
	l.output(ERROR, 0, 2, func(b []byte) []byte {
		return fmt.Appendln(b, v...)
	})
}

func (l *Logger) Flags() int {
	return int(l.flag.Load())
}

func (l *Logger) SetFlags(flag int) {
	l.flag.Store(int32(flag))
}

func (l *Logger) Prefix() string {
	if p := l.prefix.Load(); p != nil {
		return *p
	}
	return ""
}

func (l *Logger) SetPrefix(prefix string) {
	l.prefix.Store(&prefix)
}

func (l *Logger) Level() Level {
	return Level(l.minLevel.Load())
}

func (l *Logger) SetLevel(level Level) {
	l.minLevel.Store(int32(level))
}

func (l *Logger) Writer() io.Writer {
	l.outMu.Lock()
	defer l.outMu.Unlock()
	return l.out
}
