// Package httpx holds small net/http helpers shared across the app that do
// not depend on any web framework.
package httpx

import (
	"net"
	"net/http"
	"strings"
)

// unknownIP is stored when no valid address can be derived, so an INET
// NOT NULL audit column is always satisfiable.
const unknownIP = "0.0.0.0"

// ClientIP returns the caller's address for audit purposes: the last
// X-Forwarded-For hop (nginx is the sole in-path proxy and appends the
// address it saw), else the connection's RemoteAddr host. It is a
// best-effort breadcrumb, not a security control — the backend port is
// also published directly, so the header can be forged (see
// docs/KNOWN_LIMITATIONS.md).
func ClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		hops := strings.Split(xff, ",")
		if ip := net.ParseIP(strings.TrimSpace(hops[len(hops)-1])); ip != nil {
			return ip.String()
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.String()
	}
	return unknownIP
}
