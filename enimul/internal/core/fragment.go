package core

import (
	"bytes"
	"encoding/binary"
	"net"
	"strconv"
	"time"

	E "github.com/lzpls/enimul/internal/errors"
)

func findLastDotOrMidPos(data []byte, sniStart, sniLen int) int {
	subIdx := bytes.LastIndexByte(data[sniStart:sniStart+sniLen], '.')
	if subIdx == -1 {
		return sniLen/2 + sniStart
	}
	return sniStart + subIdx
}

func sendRecords(conn net.Conn, clientHello []byte,
	offset, length, records, segments int, minorVersion Byte,
	oob, oobex, waitForAckEnabled bool,
	interval time.Duration) error {
	if !minorVersion.IsUnset() {
		clientHello[2] = minorVersion.Byte()
	}

	if records == 1 {
		if oobex {
			if err := sendWithOOB(conn, clientHello[:15], clientHello[15]); err != nil {
				return E.WithStr("oob 1", err)
			}
			if err := sendWithOOB(conn, clientHello[16:35], 0x0); err != nil {
				return E.WithStr("oob 2", err)
			}
			if err := waitForAck(waitForAckEnabled, conn, interval); err != nil {
				return err
			}
			clientHello = clientHello[35:]
			offset -= 35
		}
		if segments == 1 {
			if _, err := conn.Write(clientHello); err != nil {
				return E.WithStr("send remaining data", err)
			}
			return nil
		}
		var leftSegments, rightSegments int
		leftSegments = segments / 2
		rightSegments = segments - leftSegments
		packets := make([][]byte, 0, segments)
		cut := findLastDotOrMidPos(clientHello, offset, length)
		splitAndAppend(clientHello[:cut], nil, leftSegments, &packets)
		splitAndAppend(clientHello[cut:], nil, rightSegments, &packets)
		for i, packet := range packets {
			if i == 0 && oob {
				if err := sendWithOOB(conn, packet, 0x0); err != nil {
					return E.WithStr("oob", err)
				}
			} else {
				if _, err := conn.Write(packet); err != nil {
					return E.WithStr("write packet "+strconv.Itoa(i+1), err)
				}
			}
			if err := waitForAck(waitForAckEnabled, conn, interval); err != nil {
				return err
			}
		}
		return nil
	}

	leftChunks := records / 2
	rightChunks := records - leftChunks
	chunks := make([][]byte, 0, records)
	cut := findLastDotOrMidPos(clientHello, offset, length)
	header := clientHello[:3]
	splitAndAppend(clientHello[tlsRecordHeaderLen:cut], header, leftChunks, &chunks)
	splitAndAppend(clientHello[cut:], header, rightChunks, &chunks)

	if segments == -1 {
		for i, chunk := range chunks {
			if i == 0 {
				if oob {
					if err := sendWithOOB(conn, chunk, 0x0); err != nil {
						return E.WithStr("oob", err)
					}
				} else if oobex {
					l := len(chunk)
					if err := sendWithOOB(conn, chunk[:l-1], chunk[l-1]); err != nil {
						return E.WithStr("oob 1", err)
					}
				}
			} else if i == 1 && oobex {
				if err := sendWithOOB(conn, chunk, 0x0); err != nil {
					return E.WithStr("oob 2", err)
				}
			} else {
				if _, err := conn.Write(chunk); err != nil {
					return E.WithStr("write record "+strconv.Itoa(i+1), err)
				}
			}
			if err := waitForAck(waitForAckEnabled, conn, interval); err != nil {
				return err
			}
		}
		return nil
	}

	merged := make([]byte, 0, records*tlsRecordHeaderLen+len(clientHello))
	for _, c := range chunks {
		merged = append(merged, c...)
	}

	if oobex {
		if err := sendWithOOB(conn, merged[:15], merged[15]); err != nil {
			return E.WithStr("oob 1", err)
		}
		if err := sendWithOOB(conn, merged[16:35], 0x0); err != nil {
			return E.WithStr("oob 2", err)
		}
		if err := waitForAck(waitForAckEnabled, conn, interval); err != nil {
			return err
		}
		merged = merged[35:]
	}
	if segments == 1 || len(merged) <= segments {
		_, err := conn.Write(merged)
		return err
	}

	base := len(merged) / segments
	for i := range segments {
		start := i * base
		end := start + base
		if i == segments-1 {
			end = len(merged)
		}
		if i == 0 && oob {
			if err := sendWithOOB(conn, merged[start:end], 0x0); err != nil {
				return E.WithStr("oob", err)
			}
		} else {
			if _, err := conn.Write(merged[start:end]); err != nil {
				return E.WithStr("write segment "+strconv.Itoa(i+1), err)
			}
		}
		if err := waitForAck(waitForAckEnabled, conn, interval); err != nil {
			return err
		}
	}
	return nil
}

func splitAndAppend(data, header []byte, n int, result *[][]byte) {
	if n <= 0 {
		panic("impossible")
	}
	addHeader := header != nil
	if n == 1 || len(data) < n {
		if addHeader {
			*result = append(*result, makeRecord(header, data))
		} else {
			*result = append(*result, data)
		}
		return
	}
	base := len(data) / n
	for i := range n {
		var part []byte
		if i == n-1 {
			part = data[i*base:]
		} else {
			part = data[i*base : (i+1)*base]
		}
		if addHeader {
			*result = append(*result, makeRecord(header, part))
		} else {
			*result = append(*result, part)
		}
	}
}

func makeRecord(header, payload []byte) []byte {
	rec := make([]byte, 5+len(payload))
	copy(rec[:3], header)
	binary.BigEndian.PutUint16(rec[3:5], uint16(len(payload)))
	copy(rec[5:], payload)
	return rec
}
