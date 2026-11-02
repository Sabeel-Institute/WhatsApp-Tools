package server

import (
	"crypto/subtle"
	"net"
	"net/http"
	"strings"
)

// The app's UI is a web page served on 127.0.0.1, which any website the user
// visits could also try to talk to. Three checks keep other sites out:
//
//  1. Host must be 127.0.0.1:<port> or localhost:<port>. A DNS-rebinding
//     attack reaches us under the attacker's host name and fails here.
//  2. Cross-site requests are refused (Origin and Sec-Fetch-Site).
//  3. Every /api request carries the random session token from the URL the
//     app opened. Other sites cannot read it, and sending a custom header
//     cross-origin requires a CORS preflight that this server never grants.
const tokenHeader = "X-App-Token"

func (s *Server) allowedHost(host string) bool {
	h, port, err := net.SplitHostPort(host)
	if err != nil || port != s.portStr() {
		return false
	}
	return h == "127.0.0.1" || h == "localhost"
}

func (s *Server) allowedOrigin(origin string) bool {
	if origin == "" {
		return true
	}
	for _, o := range s.cfg.DevOrigins {
		if origin == o {
			return true
		}
	}
	p := s.portStr()
	return origin == "http://127.0.0.1:"+p || origin == "http://localhost:"+p
}

func (s *Server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.allowedHost(r.Host) {
			http.Error(w, "forbidden host", http.StatusForbidden)
			return
		}
		if !s.allowedOrigin(r.Header.Get("Origin")) || r.Header.Get("Sec-Fetch-Site") == "cross-site" {
			http.Error(w, "cross-origin requests are not allowed", http.StatusForbidden)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			got := r.Header.Get(tokenHeader)
			if subtle.ConstantTimeCompare([]byte(got), []byte(s.cfg.Token)) != 1 {
				http.Error(w, "missing or wrong session token", http.StatusUnauthorized)
				return
			}
			s.touch()
		}
		next.ServeHTTP(w, r)
	})
}
