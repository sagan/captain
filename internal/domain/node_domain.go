package domain

import (
	"errors"
	"net"
	"strings"

	"golang.org/x/net/idna"
)

var ErrNodeDomain = errors.New("domain must be one host name without a scheme, port, path or wildcard")

// NormalizeNodeDomain treats DNS case, the root dot and IDNA spellings alike.
// This is only the node's public host name, not a TLS/REALITY server name.
func NormalizeNodeDomain(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	name, err := idna.Lookup.ToASCII(raw)
	if err != nil {
		return "", ErrNodeDomain
	}
	name = strings.TrimSuffix(strings.ToLower(name), ".")
	if len(name) > 253 || !strings.Contains(name, ".") || net.ParseIP(name) != nil {
		return "", ErrNodeDomain
	}
	for _, label := range strings.Split(name, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", ErrNodeDomain
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return "", ErrNodeDomain
			}
		}
	}
	return name, nil
}

// NodeDomainKey also compares legacy values without changing stored data.
func NodeDomainKey(raw string) string {
	if name, err := NormalizeNodeDomain(raw); err == nil {
		return name
	}
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(raw)), ".")
}
