package obfs

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	mRand "math/rand"
	"net"
	"net/http"

	"github.com/xjasonlyu/tun2socks/v2/buffer"
)

// maxHeaderBytes bounds the HTTP response header search so that a malformed
// or malicious response cannot make the client buffer indefinitely.
const maxHeaderBytes = 8 << 10 // 8 KiB

// HTTPObfs is shadowsocks http simple-obfs implementation
type HTTPObfs struct {
	net.Conn
	host          string
	port          string
	buf           []byte
	offset        int
	firstRequest  bool
	firstResponse bool
}

func (ho *HTTPObfs) Read(b []byte) (int, error) {
	if ho.buf != nil {
		n := copy(b, ho.buf[ho.offset:])
		ho.offset += n
		if ho.offset == len(ho.buf) {
			buffer.Put(ho.buf)
			ho.buf = nil
		}
		return n, nil
	}

	if ho.firstResponse {
		// The response header may span multiple TCP segments; keep
		// reading until the \r\n\r\n delimiter appears, bounded by
		// maxHeaderBytes. Returning on the first segment without the
		// delimiter (the old behavior) drops healthy connections.
		buf := buffer.Get(buffer.RelayBufferSize)
		total := 0
		idx := -1
		for {
			n, err := ho.Conn.Read(buf[total:])
			total += n
			idx = bytes.Index(buf[:total], []byte("\r\n\r\n"))
			if idx != -1 {
				if idx+4 > maxHeaderBytes {
					buffer.Put(buf)
					return 0, fmt.Errorf(
						"http obfs: response header exceeds %d bytes", maxHeaderBytes)
				}
				break
			}
			if err != nil {
				buffer.Put(buf)
				return 0, err
			}
			if total >= maxHeaderBytes {
				buffer.Put(buf)
				return 0, fmt.Errorf(
					"http obfs: response header exceeds %d bytes", maxHeaderBytes)
			}
		}
		ho.firstResponse = false
		length := total - (idx + 4)
		n := copy(b, buf[idx+4:total])
		if length > n {
			ho.buf = buf[:total]
			ho.offset = idx + 4 + n
		} else {
			buffer.Put(buf)
		}
		return n, nil
	}
	return ho.Conn.Read(b)
}

func (ho *HTTPObfs) Write(b []byte) (int, error) {
	if ho.firstRequest {
		randBytes := make([]byte, 16)
		rand.Read(randBytes)
		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://%s/", ho.host), bytes.NewBuffer(b))
		if err != nil {
			return 0, fmt.Errorf("http obfs: build request: %w", err)
		}
		req.Header.Set("User-Agent", fmt.Sprintf("curl/7.%d.%d", mRand.Int()%54, mRand.Int()%2))
		req.Header.Set("Upgrade", "websocket")
		req.Header.Set("Connection", "Upgrade")
		req.Host = ho.host
		if ho.port != "80" {
			req.Host = fmt.Sprintf("%s:%s", ho.host, ho.port)
		}
		req.Header.Set("Sec-WebSocket-Key", base64.URLEncoding.EncodeToString(randBytes))
		req.ContentLength = int64(len(b))
		err = req.Write(ho.Conn)
		ho.firstRequest = false
		return len(b), err
	}

	return ho.Conn.Write(b)
}

// NewHTTPObfs return a HTTPObfs. It validates the obfs host at construction
// time so that an unusable host fails fast instead of panicking on the first
// write (http.NewRequest returns nil on invalid URLs).
func NewHTTPObfs(conn net.Conn, host string, port string) (net.Conn, error) {
	if host == "" {
		return nil, fmt.Errorf("http obfs: obfs-host is empty")
	}
	if _, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://%s/", host), nil); err != nil {
		return nil, fmt.Errorf("http obfs: invalid obfs-host %q: %w", host, err)
	}

	return &HTTPObfs{
		Conn:          conn,
		firstRequest:  true,
		firstResponse: true,
		host:          host,
		port:          port,
	}, nil
}
