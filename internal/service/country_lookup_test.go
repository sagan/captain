package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type countryTransport func(*http.Request) (*http.Response, error)

func (f countryTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestCountryLookupValidatesResponses(t *testing.T) {
	for _, tc := range []struct {
		name, body, country string
		status              int
		bad                 bool
	}{
		{"ok", `{"ip":"198.51.100.20","country":"JP"}`, "JP", 200, false},
		{"wrong-IP", `{"ip":"203.0.113.30","country":"JP"}`, "", 200, true},
		{"invalid-country", `{"ip":"198.51.100.20","country":"<script>"}`, "", 200, true},
		{"unknown", `{"ip":"198.51.100.20","country":null}`, "", 200, false},
		{"oversize", strings.Repeat("x", 5000), "", 200, true},
		{"not-found", ``, "", 404, false}, {"limited", ``, "", 429, true}, {"invalid-json", `{`, "", 200, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := &http.Client{Transport: countryTransport(func(r *http.Request) (*http.Response, error) {
				if r.URL.String() != "https://api.country.is/198.51.100.20" || r.Header.Get("Authorization") != "" {
					t.Fatal(r.URL)
				}
				return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body)), Header: make(http.Header)}, nil
			})}
			code, err := lookupCountryWithClient(context.Background(), "198.51.100.20", c)
			if (err != nil) != tc.bad || code != tc.country {
				t.Fatal(code, err)
			}
		})
	}
	c := &http.Client{Transport: countryTransport(func(*http.Request) (*http.Response, error) { t.Fatal("private/invalid address sent"); return nil, nil })}
	for _, ip := range []string{"127.0.0.1", "10.10.0.2", "100.64.0.1", "fe80::1", "example.com", "https://example.com"} {
		if _, err := lookupCountryWithClient(context.Background(), ip, c); err == nil {
			t.Fatal(ip)
		}
	}
}
