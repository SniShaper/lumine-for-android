package lumine

import (
	"encoding/binary"
	"net"
	"strconv"
	"time"
)

func sendRecords(conn net.Conn, clientHello []byte,
	offset, length, records, segments int,
	oob, oobex, modMinorVer bool, interval time.Duration) error {
	if modMinorVer {
		clientHello[2] = 0x04
	}

	if records == 1 {
		if oobex {
			if err := sendWithOOB(conn, clientHello[:15], clientHello[15]); err != nil {
				return wrap("oob 1", err)
			}
			if err := sendWithOOB(conn, clientHello[16:35], 0x0); err != nil {
				return wrap("oob 2", err)
			}
			if interval > 0 {
				time.Sleep(interval)
			}
			clientHello = clientHello[35:]
			offset -= 35
		}
		if segments == 1 {
			if _, err := conn.Write(clientHello); err != nil {
				return wrap("send remaining data", err)
			}
			return nil
		}
		var leftSegments, rightSegments int
		leftSegments = segments / 2
		rightSegments = segments - leftSegments
		packets := make([][]byte, 0, segments)
		cut, _ := findLastDot(clientHello, offset, length)
		splitAndAppend(clientHello[:cut], nil, leftSegments, &packets)
		splitAndAppend(clientHello[cut:], nil, rightSegments, &packets)
		for i, packet := range packets {
			if i == 0 && oob {
				if err := sendWithOOB(conn, packet, 0x0); err != nil {
					return wrap("oob", err)
				}
			} else {
				if _, err := conn.Write(packet); err != nil {
					return wrap("write packet "+strconv.Itoa(i+1), err)
				}
			}
			if interval > 0 {
				time.Sleep(interval)
			}
		}
		return nil
	}

	leftChunks := records / 2
	rightChunks := records - leftChunks
	chunks := make([][]byte, 0, records)
	cut, _ := findLastDot(clientHello, offset, length)
	header := clientHello[:3]
	splitAndAppend(clientHello[5:cut], header, leftChunks, &chunks)
	splitAndAppend(clientHello[cut:], header, rightChunks, &chunks)

	if segments == -1 {
		for i, chunk := range chunks {
			if i == 0 {
				if oob {
					if err := sendWithOOB(conn, chunk, 0x0); err != nil {
						return wrap("oob", err)
					}
					if interval > 0 {
						time.Sleep(interval)
					}
				} else if oobex {
					l := len(chunk)
					if err := sendWithOOB(conn, chunk[:l-1], chunk[l-1]); err != nil {
						return wrap("oob 1", err)
					}
				}
			} else if i == 1 && oobex {
				if err := sendWithOOB(conn, chunk, 0x0); err != nil {
					return wrap("oob 2", err)
				}
			} else {
				if _, err := conn.Write(chunk); err != nil {
					return wrap("write record "+strconv.Itoa(i+1), err)
				}
				if interval > 0 {
					time.Sleep(interval)
				}
			}
		}
		return nil
	}

	merged := make([]byte, 0, records*5+len(clientHello))
	for _, c := range chunks {
		merged = append(merged, c...)
	}

	if oobex {
		if err := sendWithOOB(conn, merged[:15], merged[15]); err != nil {
			return wrap("oob 1", err)
		}
		if err := sendWithOOB(conn, merged[16:35], 0x0); err != nil {
			return wrap("oob 2", err)
		}
		if interval > 0 {
			time.Sleep(interval)
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
				return wrap("oob", err)
			}
		} else {
			if _, err := conn.Write(merged[start:end]); err != nil {
				return wrap("write segment "+strconv.Itoa(i+1), err)
			}
		}
		if interval > 0 {
			time.Sleep(interval)
		}
	}
	return nil
}

func splitAndAppend(data, header []byte, n int, result *[][]byte) {
	if n <= 0 {
		return
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
