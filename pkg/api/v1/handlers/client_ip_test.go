package handlers

import (
	"net/http"
	"testing"
)

func TestClientIP(t *testing.T) {
	tests := []struct {
		name       string
		xff        string
		xRealIP    string
		remoteAddr string
		want       string
	}{
		{
			name:       "x-forwarded-for leftmost wins",
			xff:        "203.0.113.7, 10.0.0.1, 10.0.0.2",
			xRealIP:    "198.51.100.9",
			remoteAddr: "10.0.0.254:54321",
			want:       "203.0.113.7",
		},
		{
			name:       "x-real-ip when no xff",
			xRealIP:    "198.51.100.9",
			remoteAddr: "10.0.0.254:54321",
			want:       "198.51.100.9",
		},
		{
			name:       "remote addr port stripped",
			remoteAddr: "192.0.2.5:443",
			want:       "192.0.2.5",
		},
		{
			name:       "remote addr without port",
			remoteAddr: "192.0.2.5",
			want:       "192.0.2.5",
		},
		{
			name:       "empty xff falls through",
			xff:        "  ",
			remoteAddr: "192.0.2.5:443",
			want:       "192.0.2.5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &http.Request{Header: http.Header{}, RemoteAddr: tt.remoteAddr}
			if tt.xff != "" {
				r.Header.Set("X-Forwarded-For", tt.xff)
			}
			if tt.xRealIP != "" {
				r.Header.Set("X-Real-IP", tt.xRealIP)
			}
			if got := clientIP(r); got != tt.want {
				t.Errorf("clientIP() = %q, want %q", got, tt.want)
			}
		})
	}
}
