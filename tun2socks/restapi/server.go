package restapi

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/go-chi/render"
	"github.com/gorilla/websocket"

	V "github.com/xjasonlyu/tun2socks/v2/internal/version"
	"github.com/xjasonlyu/tun2socks/v2/tunnel/statistic"
)

var (
	_upgrader = websocket.Upgrader{
		// D5: validate Origin on WebSocket upgrade to prevent cross-site hijacking.
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				// Non-browser clients (curl, etc.) — allow
				return true
			}
			// Allow same-origin requests
			host := r.Host
			return strings.HasPrefix(origin, "http://"+host) || strings.HasPrefix(origin, "https://"+host)
		},
	}

	_endpoints = make(map[string]http.Handler)

	// _listener tracks the running API listener so that it can be closed
	// on engine stop/restart instead of leaking.
	_listenerMu sync.Mutex
	_listener   net.Listener
)

func registerEndpoint(pattern string, handler http.Handler) {
	_endpoints[pattern] = handler
}

func Start(addr, token string) error {
	// D5: refuse to start if binding to a non-loopback address without a token,
	// as that would expose an unauthenticated management API to the network.
	if token == "" && !isLoopbackAddr(addr) {
		return fmt.Errorf("restapi: refusing to start on non-loopback %s without authentication token", addr)
	}

	r := chi.NewRouter()

	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
		MaxAge:         300,
	})

	r.Use(c.Handler)
	r.Group(func(r chi.Router) {
		r.Use(authenticator(token))
		r.Get("/", hello)
		r.Get("/traffic", traffic)
		r.Get("/version", version)
		// attach HTTP handlers
		for pattern, handler := range _endpoints {
			r.Mount(pattern, handler)
		}
	})

	// A listener left over from a previous engine run would keep serving
	// stale state (and hold the port): close it first.
	_listenerMu.Lock()
	if _listener != nil {
		_ = _listener.Close()
		_listener = nil
	}
	_listenerMu.Unlock()

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	_listenerMu.Lock()
	_listener = listener
	_listenerMu.Unlock()

	// D5: enforce read/write/idle timeouts to prevent slowloris-style
	// connection pinning and resource exhaustion.
	srv := &http.Server{
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	return srv.Serve(listener)
}

// Stop closes the running REST API listener, if any. It is safe to call
// multiple times and is a no-op when the API is not running.
func Stop() {
	_listenerMu.Lock()
	defer _listenerMu.Unlock()
	if _listener != nil {
		_ = _listener.Close()
		_listener = nil
	}
}

func hello(w http.ResponseWriter, r *http.Request) {
	render.JSON(w, r, render.M{"hello": V.Name})
}

func authenticator(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			if token == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Browser websocket not support custom header
			if websocket.IsWebSocketUpgrade(r) && r.URL.Query().Get("token") != "" {
				t := r.URL.Query().Get("token")
				// D5: constant-time comparison to prevent timing side-channels
				if subtle.ConstantTimeCompare([]byte(t), []byte(token)) != 1 {
					render.Status(r, http.StatusUnauthorized)
					render.JSON(w, r, ErrUnauthorized)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			header := r.Header.Get("Authorization")
			text := strings.SplitN(header, " ", 2)

			hasInvalidHeader := text[0] != "Bearer"
			// D5: constant-time comparison for bearer token
			hasInvalidToken := len(text) != 2 || subtle.ConstantTimeCompare([]byte(text[1]), []byte(token)) != 1
			if hasInvalidHeader || hasInvalidToken {
				render.Status(r, http.StatusUnauthorized)
				render.JSON(w, r, ErrUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}

func traffic(w http.ResponseWriter, r *http.Request) {
	var (
		err    error
		wsConn *websocket.Conn
	)
	if websocket.IsWebSocketUpgrade(r) {
		wsConn, err = _upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer wsConn.Close()
	}

	if wsConn == nil {
		w.Header().Set("Content-Type", "application/json")
		render.Status(r, http.StatusOK)
	}

	tick := time.NewTicker(time.Second)
	defer tick.Stop()

	buf := &bytes.Buffer{}
	for range tick.C {
		buf.Reset()

		up, down := statistic.DefaultManager.Now()
		if err = json.NewEncoder(buf).Encode(struct {
			Up   int64 `json:"up"`
			Down int64 `json:"down"`
		}{
			Up:   up,
			Down: down,
		}); err != nil {
			break
		}

		if wsConn == nil {
			_, err = w.Write(buf.Bytes())
			w.(http.Flusher).Flush()
		} else {
			err = wsConn.WriteMessage(websocket.TextMessage, buf.Bytes())
		}

		if err != nil {
			break
		}
	}
}

func version(w http.ResponseWriter, r *http.Request) {
	render.JSON(w, r, render.M{
		"version": V.Version,
		"commit":  V.GitCommit,
		"modules": V.Info(),
	})
}

// isLoopbackAddr checks whether the given network address (host:port) resolves
// to a loopback interface. Used by D5 to enforce that an unauthenticated REST
// API must only bind to loopback.
func isLoopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// addr may be bare host without port; treat as-is
		host = addr
	}
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return true
	}
	ip := net.ParseIP(host)
	if ip != nil {
		return ip.IsLoopback()
	}
	// Resolve hostname and check all resulting IPs
	addrs, err := net.LookupHost(host)
	if err != nil {
		return false
	}
	for _, a := range addrs {
		ip = net.ParseIP(a)
		if ip == nil || !ip.IsLoopback() {
			return false
		}
	}
	return len(addrs) > 0
}
