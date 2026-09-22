package httpx

import (
	"net/http/httptest"
	"testing"
)

func TestClientIP(t *testing.T) {
	tests := []struct {
		name       string
		xff        string
		remoteAddr string
		want       string
	}{
		{"last forwarded hop wins", "203.0.113.9, 10.0.0.2, 198.51.100.7", "172.18.0.5:4321", "198.51.100.7"},
		{"single forwarded hop", "203.0.113.9", "172.18.0.5:4321", "203.0.113.9"},
		{"garbage header falls back to remote addr", "not-an-ip", "172.18.0.5:4321", "172.18.0.5"},
		{"no header uses remote addr host", "", "192.0.2.1:1234", "192.0.2.1"},
		{"ipv6 remote addr", "", "[2001:db8::1]:80", "2001:db8::1"},
		{"remote addr without port", "", "192.0.2.1", "192.0.2.1"},
		{"nothing usable", "", "@", "0.0.0.0"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			r.RemoteAddr = tc.remoteAddr
			if tc.xff != "" {
				r.Header.Set("X-Forwarded-For", tc.xff)
			}
			if got := ClientIP(r); got != tc.want {
				t.Errorf("ClientIP() = %q, want %q", got, tc.want)
			}
		})
	}
}
