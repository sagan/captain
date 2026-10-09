package domain

import (
	"strings"
	"testing"
)

func TestNormalizeNodeDomain(t *testing.T) {
	for raw, want := range map[string]string{
		"": "", "  JP1.Example.COM. ": "jp1.example.com",
		"bücher.example.com":         "xn--bcher-kva.example.com",
		"XN--BCHER-KVA.example.com.": "xn--bcher-kva.example.com",
		"jp1。example.com":            "jp1.example.com",
	} {
		got, err := NormalizeNodeDomain(raw)
		if err != nil || got != want {
			t.Errorf("%q: %q %v", raw, got, err)
		}
	}
	for _, raw := range []string{"https://jp1.example.com", "jp1.example.com:443", "jp1.example.com/path", "a.example.com,b.example.com", "a.example.com b.example.com", "a.example.com\nb.example.com", "*.example.com", "192.0.2.10", "2001:db8::10", "localhost", "-x.example.com", "x_.example.com", "x.example.com..", strings.Repeat("x", 64) + ".example.com"} {
		if _, err := NormalizeNodeDomain(raw); err == nil {
			t.Errorf("accepted %q", raw)
		}
	}
}
