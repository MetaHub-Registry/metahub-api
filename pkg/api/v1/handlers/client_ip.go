package handlers

import (
	"net"
	"net/http"
	"strings"
)

// clientIP returns the originating client IP for a request. Because the gateway
// proxies to the backend over h2c, r.RemoteAddr is always the gateway address;
// the real client lives in the forwarded headers. Resolution order:
//
//  1. leftmost entry of X-Forwarded-For (the original client; subsequent
//     entries are intermediate proxies),
//  2. X-Real-IP,
//  3. r.RemoteAddr with the port stripped.
//
// It returns an empty string only when no source can be determined.
//
// Trust caveat: the leftmost X-Forwarded-For entry is client-spoofable. The
// Envoy Gateway fronting metahub-api has no ClientTrafficPolicy/numTrustedHops
// configured, and on the public path (*.metahub-registry.org) traffic also
// passes through Pangolin (an extra proxy hop), so a client can forge the
// leftmost value. That is acceptable here because client_ip is used only for
// usage reporting, never for authorization. To make it trustworthy, add an
// Envoy Gateway ClientTrafficPolicy and consume the gateway-computed
// x-envoy-external-address header instead of the raw leftmost entry.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		first := strings.TrimSpace(strings.SplitN(xff, ",", 2)[0])
		if first != "" {
			return first
		}
	}
	if xrip := strings.TrimSpace(r.Header.Get("X-Real-IP")); xrip != "" {
		return xrip
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
