package core

import (
	"context"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"sync"

	"github.com/lzpls/enimul/internal/dial"
	F "github.com/lzpls/enimul/internal/fmt"
	"github.com/lzpls/enimul/internal/log"
)

const (
	status500 = "500 Internal Server Error"
	status403 = "403 Forbidden"
	status502 = "502 Bad Gateway"
)

var (
	httpConnID      uint32
	httpConnIDMutex sync.Mutex
)

func getHTTPConnID() uint32 {
	httpConnIDMutex.Lock()
	defer httpConnIDMutex.Unlock()
	httpConnID++
	if httpConnID > maxConnID {
		httpConnID = 1
	}
	return httpConnID
}

func HTTPAccept(addr *string, serverAddr string) {
	var listenAddr string
	if *addr == "" {
		listenAddr = serverAddr
	} else {
		listenAddr = *addr
	}
	if listenAddr == "" {
		fmt.Println("HTTP bind address is not specified")
		return
	}
	if listenAddr == "none" {
		return
	}

	logger := newLogger("H[00000]")
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		logger.Error("Failed to start HTTP proxy server: ", err)
		return
	}
	logger.Info("HTTP proxy server started at ", ln.Addr())
	if err := http.Serve(ln, http.HandlerFunc(httpHandler)); err != nil {
		logger.Error("HTTP serve: ", err)
	}
}

func httpHandler(w http.ResponseWriter, req *http.Request) {
	logger := newLogger(F.ConnIDToHex5('H', getHTTPConnID()))
	logger.Info(req.RemoteAddr, " - \"", req.Method, " ", req.RequestURI, " ", req.Proto, "\"")

	if req.Method == http.MethodConnect {
		handleConnect(logger, w, req)
		return
	}

	if !req.URL.IsAbs() {
		logger.Error("URI not fully qualified")
		http.Error(w, status403, http.StatusForbidden)
		return
	}

	forwardHTTPRequest(logger, w, req)
}

func handleConnect(logger log.Logger, w http.ResponseWriter, req *http.Request) {
	oldDest := req.Host
	if oldDest == "" {
		logger.Error("Empty host")
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	originHost, originPort, err := net.SplitHostPort(oldDest)
	if err != nil {
		logger.Error("Split ", oldDest, ": ", err)
		return
	}

	dstHost, policy, fail, blocked, _ := genPolicy(logger, originHost, false, false)
	if fail {
		http.Error(w, status500, http.StatusInternalServerError)
		return
	}
	if blocked {
		logger.Info("Connection blocked")
		http.Error(w, status403, http.StatusForbidden)
		return
	}

	logger.Info("Policy: ", policy)

	if policy.Mode == ModeBlock {
		http.Error(w, status403, http.StatusForbidden)
		return
	}

	dstPort := originPort
	if policy.Port != 0 && policy.Port != unsetInt {
		dstPort = F.Int(policy.Port)
	}

	dest := net.JoinHostPort(dstHost, dstPort)

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		logger.Error("Hijacking not supported")
		http.Error(w, status500, http.StatusInternalServerError)
		return
	}
	cliConn, _, err := hijacker.Hijack()
	if err != nil {
		logger.Error("Hijack: ", err)
		http.Error(w, status500, http.StatusInternalServerError)
		return
	}

	var (
		closeHere = true
		dstConn   net.Conn
	)
	defer func() {
		if closeHere {
			cliConn.Close()
		}
	}()

	if policy.ReplyFirst != BoolTrue {
		dstConn, err = dial.DialTCPTimeout(dest, policy.ConnectTimeout)
		if err != nil {
			logger.Error("Connection to ", oldDest, " failed: ", err)
			_, err = cliConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
			if err != nil {
				logger.Error("Send 502: ", err)
			}
			return
		}
	}
	_, err = cliConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	if err != nil {
		logger.Error("Send 200: ", err)
		return
	}

	closeHere = false
	handleTunnel(policy, dstConn, cliConn, logger, oldDest, dest, originHost, originPort)
}

func forwardHTTPRequest(logger log.Logger, w http.ResponseWriter, originReq *http.Request) {
	host := originReq.Host
	if host == "" {
		host = originReq.URL.Host
		if host == "" {
			logger.Error("Cannot determine target host")
			http.Error(w, "400 Bad Request", http.StatusBadRequest)
			return
		}
	}

	originHost, port, err := net.SplitHostPort(host)
	if err != nil {
		originHost = host
		if originReq.URL.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}

	dstHost, p, failed, blocked, _ := genPolicy(logger, originHost, false, false)
	if failed {
		http.Error(w, status500, http.StatusInternalServerError)
		return
	}
	if blocked {
		logger.Info("Connection blocked")
		http.Error(w, status403, http.StatusForbidden)
		return
	}

	if p.HttpStatus != 0 && p.HttpStatus != unsetInt {
		if p.HttpStatus == 301 || p.HttpStatus == 302 {
			scheme := originReq.URL.Scheme
			if scheme == "" {
				scheme = "https"
			}
			location := scheme + "://" + host + originReq.URL.RequestURI()
			w.Header().Set("Location", location)
		}
		w.WriteHeader(p.HttpStatus)
		logger.Info("Sent ", p.HttpStatus, " ", http.StatusText(p.HttpStatus))
		return
	}

	dstPort := port
	if p.Port != 0 && p.Port != unsetInt {
		dstPort = F.Int(p.Port)
	}

	outReq := originReq.Clone(context.Background())

	targetAddr := net.JoinHostPort(dstHost, dstPort)
	outReq.URL.Host = targetAddr
	outReq.Host = targetAddr

	if outReq.URL.Scheme == "" {
		outReq.URL.Scheme = "http"
	}

	outReq.Header.Del("Proxy-Authorization")
	outReq.Header.Del("Proxy-Connection")
	if outReq.Header.Get("Connection") == "" {
		outReq.Header.Set("Connection", "close")
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()

	if p.ConnectTimeout > 0 {
		transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dial.DialTimeout(ctx, network, addr, p.ConnectTimeout)
		}
	}

	resp, err := transport.RoundTrip(outReq)
	if err != nil {
		logger.Error("Transport: ", err)
		http.Error(w, status502, http.StatusBadGateway)
		return
	}
	logger.Info("Start forwarding")
	defer resp.Body.Close()

	maps.Copy(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)

	if _, err = io.Copy(w, resp.Body); err != nil {
		logger.Error("Copy response body: ", err)
	}
}
