package contract

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	local "github.com/sam-cogan/external-dns-traffic-manager/pkg/provider"
	"go.uber.org/zap"
	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/pkg/apis/externaldns"
	"sigs.k8s.io/external-dns/plan"
	"sigs.k8s.io/external-dns/provider/webhook"
	"sigs.k8s.io/external-dns/source/annotations"
)

type backend struct {
	records []*local.Endpoint
	changes *local.Changes
	err     error
}

func (b *backend) GetDomainFilter() local.DomainFilter {
	return local.DomainFilter{Include: []string{"example.com"}, Exclude: []string{"private.example.com"}}
}
func (b *backend) Records(context.Context) ([]*local.Endpoint, error) { return b.records, b.err }
func (b *backend) ApplyChanges(_ context.Context, changes *local.Changes) error {
	b.changes = changes
	return b.err
}
func (b *backend) AdjustEndpoints(_ context.Context, endpoints []*local.Endpoint) []*local.Endpoint {
	return endpoints
}

func TestRealExternalDNSClient(t *testing.T) {
	b := &backend{}
	server := local.NewWebhookServer(b, zap.NewNop())
	mux := http.NewServeMux()
	mux.HandleFunc("/", server.HandleNegotiate)
	mux.HandleFunc("/records", server.HandleRecords)
	mux.HandleFunc("/adjustendpoints", server.HandleAdjustEndpoints)
	var observed []string
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			if got := r.Header.Get("Content-Type"); got != "application/external.dns.webhook+json;version=1" {
				t.Errorf("upstream request content type %q", got)
			}
		}
		observed = append(observed, r.Method+" "+r.URL.Path)
		mux.ServeHTTP(w, r)
	}))
	defer httpServer.Close()
	client, err := webhook.New(t.Context(), &externaldns.Config{
		WebhookProviderURL: httpServer.URL, WebhookProviderReadTimeout: time.Second,
		WebhookProviderWriteTimeout: time.Second, WebhookProviderMaxBodySize: 1 << 20,
	}, endpoint.NewDomainFilter(nil))
	if err != nil {
		t.Fatal(err)
	}
	filter := client.GetDomainFilter()
	for domain, want := range map[string]bool{"a.example.com": true, "outside.invalid": false, "private.example.com": false} {
		if filter.Match(domain) != want {
			t.Fatalf("negotiated filter mismatch for %s", domain)
		}
	}
	records, err := client.Records(t.Context())
	if err != nil || len(records) != 0 {
		t.Fatalf("empty GET records: %v, %v", records, err)
	}
	properties, _ := annotations.ProviderSpecificAnnotations(map[string]string{
		annotations.WebhookPrefix + "traffic-manager-enabled":        "true",
		annotations.WebhookPrefix + "traffic-manager-resource-group": "test-rg",
	})
	e := endpoint.NewEndpointWithTTL("east.example.com", "A", 300, "192.0.2.1")
	e.ProviderSpecific = properties
	e.Labels = map[string]string{endpoint.OwnerLabelKey: "east"}
	e.SetIdentifier = "east"
	adjusted, err := client.AdjustEndpoints([]*endpoint.Endpoint{e})
	if err != nil || len(adjusted) != 1 || !reflect.DeepEqual(adjusted[0], e) {
		t.Fatalf("endpoint transport mismatch: %+v, %v", adjusted, err)
	}
	empty, err := client.AdjustEndpoints(nil)
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty adjustment: %v, %v", empty, err)
	}
	for _, batch := range []*plan.Changes{
		{}, {Create: []*endpoint.Endpoint{e}}, {UpdateOld: []*endpoint.Endpoint{e}, UpdateNew: []*endpoint.Endpoint{e}},
		{Delete: []*endpoint.Endpoint{e}},
	} {
		if err := client.ApplyChanges(t.Context(), batch); err != nil {
			t.Fatal(err)
		}
		if b.changes == nil {
			t.Fatal("POST did not reach provider")
		}
		for _, endpoints := range [][]*local.Endpoint{b.changes.Create, b.changes.UpdateOld, b.changes.UpdateNew, b.changes.Delete} {
			for _, got := range endpoints {
				if got.DNSName != e.DNSName || got.RecordTTL != 300 || got.SetIdentifier != "east" ||
					got.Labels[endpoint.OwnerLabelKey] != "east" || len(got.ProviderSpecific) != 2 {
					t.Fatalf("lost endpoint fields: %+v", got)
				}
				for _, property := range got.ProviderSpecific {
					if property.Name != "webhook/traffic-manager-enabled" && property.Name != "webhook/traffic-manager-resource-group" {
						t.Fatalf("unexpected wire property: %+v", property)
					}
				}
			}
		}
	}
	b.records = []*local.Endpoint{{DNSName: e.DNSName, RecordType: "A", Targets: []string{"192.0.2.1"}, RecordTTL: 300}}
	records, err = client.Records(t.Context())
	if err != nil || len(records) != 1 || records[0].DNSName != e.DNSName {
		t.Fatalf("GET records: %v, %v", records, err)
	}
	// Assert the literal wire shape too: unknown JSON fields can otherwise be
	// silently discarded by the upstream decoder.
	raw, err := json.Marshal(b.changes)
	if err != nil {
		t.Fatal(err)
	}
	var shape map[string]json.RawMessage
	if err := json.Unmarshal(raw, &shape); err != nil {
		t.Fatal(err)
	}
	if _, ok := shape["delete"]; !ok {
		t.Fatalf("wrong change field shape: %s", raw)
	}
	b.err = errors.New("backend unavailable")
	if _, err := client.Records(t.Context()); err == nil {
		t.Fatal("GET backend error hidden")
	}
	if err := client.ApplyChanges(t.Context(), &plan.Changes{}); err == nil {
		t.Fatal("POST backend error hidden")
	}
	if len(observed) < 10 {
		t.Fatalf("missing protocol calls: %v", observed)
	}
}

func TestUpstreamOwnershipSelection(t *testing.T) {
	for _, owner := range []string{"east", "west", ""} {
		current := endpoint.NewEndpoint("app.example.com", "A", "192.0.2.1")
		current.Labels = map[string]string{endpoint.OwnerLabelKey: owner}
		result := (&plan.Plan{
			Current: []*endpoint.Endpoint{current}, OwnerID: "east",
			ManagedRecords: []string{"A"}, DomainFilter: endpoint.MatchAllDomainFilters{endpoint.NewDomainFilter([]string{"example.com"})},
		}).Calculate()
		if got, want := len(result.Changes.Delete), map[bool]int{true: 1, false: 0}[owner == "east"]; got != want {
			t.Fatalf("owner %q: planned %d deletions; want %d", owner, got, want)
		}
	}
}
