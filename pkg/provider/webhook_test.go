package provider

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"
)

type protocolProvider struct {
	records []*Endpoint
	changes *Changes
	err     error
}

func (p *protocolProvider) GetDomainFilter() DomainFilter {
	return DomainFilter{Include: []string{"example.com"}}
}
func (p *protocolProvider) Records(context.Context) ([]*Endpoint, error) {
	return p.records, p.err
}
func (p *protocolProvider) ApplyChanges(_ context.Context, changes *Changes) error {
	p.changes = changes
	return p.err
}
func (p *protocolProvider) AdjustEndpoints(_ context.Context, endpoints []*Endpoint) []*Endpoint {
	return endpoints
}

func TestWebhookContract(t *testing.T) {
	p := &protocolProvider{}
	server := NewWebhookServer(p, zap.NewNop())
	mux := http.NewServeMux()
	mux.HandleFunc("/", server.HandleNegotiate)
	mux.HandleFunc("/records", server.HandleRecords)
	mux.HandleFunc("/adjustendpoints", server.HandleAdjustEndpoints)
	mux.HandleFunc("/healthz", server.HandleHealth)
	for _, tc := range []struct {
		method, path, body string
		status             int
		contains           string
	}{
		{"GET", "/", "", 200, `"include":["example.com"]`},
		{"GET", "/records", "", 200, "[]"},
		{"POST", "/records", `{"create":[{"dnsName":"a.example.com","recordType":"A","targets":["192.0.2.1"],"providerSpecific":[{"name":"webhook/traffic-manager-enabled","value":"true"}]}]}`, 204, ""},
		{"POST", "/adjustendpoints", `[]`, 200, "[]"},
		{"POST", "/adjustendpoints", `null`, 200, "[]"},
		{"POST", "/adjustendpoints", `[{"dnsName":"a.example.com","recordType":"A","targets":["192.0.2.1"],"labels":{"owner":"east"},"providerSpecific":[{"name":"webhook/test","value":"value"}]}]`, 200, `"providerSpecific":[{"name":"webhook/test","value":"value"}]`},
		{"POST", "/records", `{`, 400, "Invalid request body"},
		{"POST", "/records", `null`, 400, "changes must not be null"},
		{"POST", "/records", `{"create":[null]}`, 400, "change endpoints must not be null"},
		{"POST", "/records", `{"updateOld":[{}]}`, 400, "same length"},
		{"POST", "/adjustendpoints", `{`, 400, "Invalid request body"},
		{"DELETE", "/records", "", 405, "Method not allowed"},
		{"POST", "/", "", 405, "Method not allowed"},
		{"GET", "/adjustendpoints", "", 405, "Method not allowed"},
		{"GET", "/healthz", "", 200, `"status":"healthy"`},
		{"POST", "/healthz", "", 405, "Method not allowed"},
	} {
		t.Run(tc.method+tc.path+tc.body, func(t *testing.T) {
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
			if w.Code != tc.status || !strings.Contains(w.Body.String(), tc.contains) {
				t.Fatalf("got %d %s; want %d containing %q", w.Code, w.Body, tc.status, tc.contains)
			}
			if tc.status == 200 && tc.path != "/healthz" {
				if got := w.Header().Get("Content-Type"); got != "application/external.dns.webhook+json;version=1" {
					t.Fatalf("wrong content type: %s", got)
				}
				var response any
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatal(err)
				}
				if tc.path == "/" {
					if _, wrapped := response.(map[string]any)["domainFilter"]; wrapped {
						t.Fatal("negotiation must not wrap the domain filter")
					}
				}
			}
			if tc.status == 204 && w.Body.Len() != 0 {
				t.Fatal("204 response must be empty")
			}
		})
	}
	if p.changes == nil || p.changes.Create[0].ProviderSpecific[0].Value != "true" {
		t.Fatal("provider-specific properties lost in POST")
	}
	p.err = errors.New("backend unavailable")
	for _, method := range []string{"GET", "POST"} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(method, "/records", strings.NewReader(`{}`)))
		if w.Code != 500 {
			t.Fatalf("backend error returned %d", w.Code)
		}
	}
}
