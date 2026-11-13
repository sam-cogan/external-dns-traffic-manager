package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/sam-cogan/external-dns-traffic-manager/pkg/annotations"
	"github.com/sam-cogan/external-dns-traffic-manager/pkg/state"
	"github.com/sam-cogan/external-dns-traffic-manager/pkg/trafficmanager"
	"go.uber.org/zap"
)

type fakeTM struct {
	fail     string
	calls    []string
	profiles []*state.ProfileState
	profile  *state.ProfileState
	stateErr error

	updatedProfile      *trafficmanager.ProfileConfig
	updatedEndpoint     *trafficmanager.EndpointConfig
	createdEndpoint     *trafficmanager.EndpointConfig
	createdProfile      *trafficmanager.ProfileConfig
	deletedProfileNames []string
	deletedEndpoints    []string
}

func (f *fakeTM) call(name string) error {
	f.calls = append(f.calls, name)
	if f.fail == name {
		return errors.New("injected " + name)
	}
	return nil
}
func (f *fakeTM) SyncProfilesFromAzure(context.Context, []string) ([]*state.ProfileState, error) {
	return f.profiles, f.call("sync")
}
func (f *fakeTM) CreateProfile(_ context.Context, config *trafficmanager.ProfileConfig) (*trafficmanager.ProfileState, error) {
	f.createdProfile = config
	return &trafficmanager.ProfileState{}, f.call("create-profile")
}
func (f *fakeTM) GetProfile(context.Context, string, string) (*trafficmanager.ProfileState, error) {
	return &trafficmanager.ProfileState{}, f.call("get-profile")
}
func (f *fakeTM) UpdateProfile(_ context.Context, config *trafficmanager.ProfileConfig) (*trafficmanager.ProfileState, error) {
	f.updatedProfile = config
	return &trafficmanager.ProfileState{}, f.call("update-profile")
}
func (f *fakeTM) DeleteProfile(_ context.Context, _, profileName string) error {
	f.deletedProfileNames = append(f.deletedProfileNames, profileName)
	return f.call("delete-profile")
}
func (f *fakeTM) CreateEndpoint(_ context.Context, _, _ string, cfg *trafficmanager.EndpointConfig) (*trafficmanager.EndpointState, error) {
	f.createdEndpoint = cfg
	return &trafficmanager.EndpointState{EndpointName: cfg.EndpointName}, f.call("create-endpoint")
}
func (f *fakeTM) UpdateEndpoint(_ context.Context, _, _ string, cfg *trafficmanager.EndpointConfig) (*trafficmanager.EndpointState, error) {
	f.updatedEndpoint = cfg
	return &trafficmanager.EndpointState{EndpointName: cfg.EndpointName}, f.call("update-endpoint")
}
func (f *fakeTM) DeleteEndpoint(_ context.Context, _, profileName, _, endpointName string) error {
	f.deletedProfileNames = append(f.deletedProfileNames, profileName)
	f.deletedEndpoints = append(f.deletedEndpoints, endpointName)
	return f.call("delete-endpoint")
}
func (f *fakeTM) GetProfileState(context.Context, string, string) (*state.ProfileState, error) {
	if err := f.call("state"); err != nil {
		return nil, err
	}
	if f.stateErr != nil {
		return nil, f.stateErr
	}
	return f.profile, nil
}

type fakeDNS struct {
	err       error
	createErr error
	deleteErr error
	created   []string
	deleted   []string
}

func (f *fakeDNS) CreateOrUpdateCNAME(_ context.Context, name, _, _ string, _ int64) error {
	f.created = append(f.created, name)
	if f.createErr != nil {
		return f.createErr
	}
	return f.err
}
func (f *fakeDNS) Delete(_ context.Context, name string) error {
	f.deleted = append(f.deleted, name)
	if f.deleteErr != nil {
		return f.deleteErr
	}
	return f.err
}

func testProvider() (*TrafficManagerProvider, *fakeTM, *fakeDNS) {
	tm := &fakeTM{profile: &state.ProfileState{
		ProfileName: "profile", Hostname: "app.example.com", FQDN: "profile.trafficmanager.net",
		Endpoints: map[string]*state.EndpointState{},
	}}
	dns := &fakeDNS{}
	return &TrafficManagerProvider{
		logger: zap.NewNop(), tmClient: tm, dnsEndpointManager: dns,
		stateManager: state.NewManager(time.Minute, zap.NewNop()),
		domainFilter: []string{"example.com"},
	}, tm, dns
}

func configuredEndpoint() *Endpoint {
	return &Endpoint{DNSName: "east.example.com", RecordType: "A", Targets: []string{"192.0.2.1"},
		ProviderSpecific: []ProviderSpecificProperty{
			{Name: annotations.AnnotationEnabled, Value: "true"},
			{Name: annotations.AnnotationResourceGroup, Value: "rg"},
			{Name: annotations.AnnotationEndpointLocation, Value: "eastus"},
			{Name: annotations.AnnotationHostname, Value: "app.example.com"},
		},
	}
}

func TestConfigurationTransportAndOwnership(t *testing.T) {
	ctx := context.Background()
	p, tm, _ := testProvider()
	e := configuredEndpoint()
	e.Labels = map[string]string{annotations.AnnotationEnabled: "false"}
	if endpointAnnotations(e)[annotations.AnnotationEnabled] != "true" {
		t.Fatal("providerSpecific must override labels")
	}
	if err := p.ApplyChanges(ctx, &Changes{Create: []*Endpoint{e}}); err != nil {
		t.Fatal(err)
	}
	old := configuredEndpoint()
	e.ProviderSpecific = append(e.ProviderSpecific,
		ProviderSpecificProperty{Name: annotations.AnnotationWeight, Value: "200"},
		ProviderSpecificProperty{Name: annotations.AnnotationDNSTTL, Value: "60"})
	if err := p.ApplyChanges(ctx, &Changes{UpdateOld: []*Endpoint{old}, UpdateNew: []*Endpoint{e}}); err != nil {
		t.Fatal(err)
	}
	if err := p.ApplyChanges(ctx, &Changes{Delete: []*Endpoint{e}}); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, call := range tm.calls {
		seen[call] = true
	}
	for _, call := range []string{"create-endpoint", "update-endpoint", "update-profile", "delete-endpoint", "delete-profile"} {
		if !seen[call] {
			t.Errorf("%s was skipped: %v", call, tm.calls)
		}
	}
	tm.calls = nil
	txt := configuredEndpoint()
	txt.RecordType = "TXT"
	if err := p.ApplyChanges(ctx, &Changes{
		Create: []*Endpoint{txt}, UpdateOld: []*Endpoint{txt}, UpdateNew: []*Endpoint{txt}, Delete: []*Endpoint{txt},
	}); err != nil || len(tm.calls) != 0 {
		t.Fatalf("TXT ownership records must never mutate Azure: %v, %v", err, tm.calls)
	}
	disabled := &Endpoint{DNSName: "disabled.example.com", RecordType: "A"}
	if err := p.ApplyChanges(ctx, &Changes{
		Create: []*Endpoint{disabled}, UpdateOld: []*Endpoint{disabled}, UpdateNew: []*Endpoint{disabled}, Delete: []*Endpoint{disabled},
	}); err != nil || len(tm.calls) != 0 {
		t.Fatalf("disabled endpoints must not mutate Azure: %v, %v", err, tm.calls)
	}
}

func TestValidateChanges(t *testing.T) {
	p, tm, _ := testProvider()
	for _, changes := range []*Changes{nil, {UpdateOld: []*Endpoint{{}}}, {Create: []*Endpoint{nil}},
		{UpdateOld: []*Endpoint{nil}, UpdateNew: []*Endpoint{{}}}, {Delete: []*Endpoint{nil}}} {
		if err := p.ApplyChanges(context.Background(), changes); err == nil {
			t.Errorf("accepted invalid batch: %+v", changes)
		}
	}
	if len(tm.calls) != 0 {
		t.Fatal("invalid batches must be rejected before side effects")
	}
}

func TestProviderErrorPaths(t *testing.T) {
	for _, operation := range []string{"create", "update", "delete"} {
		for _, failure := range []string{"bad-annotations", "bad-config", "create-endpoint", "update-profile", "update-endpoint", "delete-endpoint", "delete-profile", "state", "dns"} {
			t.Run(operation+"/"+failure, func(t *testing.T) {
				p, tm, dns := testProvider()
				old, e := configuredEndpoint(), configuredEndpoint()
				e.ProviderSpecific = append(e.ProviderSpecific, ProviderSpecificProperty{Name: annotations.AnnotationWeight, Value: "200"},
					ProviderSpecificProperty{Name: annotations.AnnotationDNSTTL, Value: "60"})
				tm.fail = failure
				switch failure {
				case "bad-annotations":
					e.ProviderSpecific = append(e.ProviderSpecific, ProviderSpecificProperty{Name: annotations.AnnotationWeight, Value: "invalid"})
				case "bad-config":
					e.ProviderSpecific = append(e.ProviderSpecific, ProviderSpecificProperty{Name: annotations.AnnotationWeight, Value: "-1"})
				case "dns":
					dns.err = errors.New("dns unavailable")
				}
				var err error
				switch operation {
				case "create":
					err = p.ApplyChanges(context.Background(), &Changes{Create: []*Endpoint{e}})
				case "update":
					err = p.ApplyChanges(context.Background(), &Changes{UpdateOld: []*Endpoint{old}, UpdateNew: []*Endpoint{e}})
				case "delete":
					err = p.ApplyChanges(context.Background(), &Changes{Delete: []*Endpoint{e}})
				}
				shouldFail := failure == "bad-annotations" ||
					(failure == "bad-config" && operation != "delete") ||
					(operation == "create" && failure == "create-endpoint") ||
					(operation == "update" && (failure == "update-profile" || failure == "update-endpoint")) ||
					(operation == "delete" && (failure == "delete-endpoint" || failure == "delete-profile" || failure == "state" || failure == "dns"))
				if (err != nil) != shouldFail {
					t.Fatalf("got %v; want error=%v", err, shouldFail)
				}
			})
		}
	}
}

func TestDeletePreservesNonEmptyProfile(t *testing.T) {
	p, tm, _ := testProvider()
	tm.profile.Endpoints["west"] = &state.EndpointState{EndpointName: "west"}
	e := configuredEndpoint()
	if err := p.deleteEndpoint(context.Background(), e); err != nil {
		t.Fatal(err)
	}
	for _, call := range tm.calls {
		if call == "delete-profile" {
			t.Fatal("deleted non-empty profile")
		}
	}
	if _, ok := p.stateManager.GetProfile("app.example.com"); !ok {
		t.Fatal("remaining profile missing from state")
	}
	e.ProviderSpecific = e.ProviderSpecific[:3]
	tm.profile.Endpoints = map[string]*state.EndpointState{}
	if err := p.deleteEndpoint(context.Background(), e); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateUsesVanityHostnameAndARecordTarget(t *testing.T) {
	p, tm, _ := testProvider()
	oldEndpoint := configuredEndpoint()
	newEndpoint := configuredEndpoint()
	newEndpoint.ProviderSpecific = append(newEndpoint.ProviderSpecific,
		ProviderSpecificProperty{Name: annotations.AnnotationWeight, Value: "200"},
		ProviderSpecificProperty{Name: annotations.AnnotationRoutingMethod, Value: "Priority"})

	if err := p.updateEndpoint(context.Background(), oldEndpoint, newEndpoint); err != nil {
		t.Fatal(err)
	}
	if tm.updatedEndpoint == nil || tm.updatedEndpoint.Target != newEndpoint.DNSName {
		t.Fatalf("A-record update target = %+v, want DNS name %q", tm.updatedEndpoint, newEndpoint.DNSName)
	}
	if tm.updatedProfile == nil {
		t.Fatal("profile settings were not updated")
	}
	if got, want := tm.updatedProfile.ProfileName, "app-example-com-tm"; got != want {
		t.Errorf("profile name = %q, want %q", got, want)
	}
	if got, want := tm.updatedProfile.Tags["hostname"], "app.example.com"; got != want {
		t.Errorf("profile hostname tag = %q, want %q", got, want)
	}
	if _, ok := p.stateManager.GetProfile("app.example.com"); !ok {
		t.Fatal("updated profile was not stored under vanity hostname")
	}
	if _, ok := p.stateManager.GetProfile("east.example.com"); ok {
		t.Fatal("updated profile was also stored under service DNS name")
	}
}

func TestARecordWithMultipleTargetsUsesSingleNormalizedEndpoint(t *testing.T) {
	p, tm, _ := testProvider()
	oldEndpoint := configuredEndpoint()
	newEndpoint := configuredEndpoint()
	oldEndpoint.Targets = append(oldEndpoint.Targets, "192.0.2.2")
	newEndpoint.Targets = append(newEndpoint.Targets, "192.0.2.2")
	newEndpoint.ProviderSpecific = append(newEndpoint.ProviderSpecific,
		ProviderSpecificProperty{Name: annotations.AnnotationWeight, Value: "200"})

	if err := p.createEndpoint(context.Background(), oldEndpoint); err != nil {
		t.Fatal(err)
	}
	if tm.createdEndpoint == nil || tm.createdEndpoint.EndpointName != "192-0-2-1" {
		t.Fatalf("created endpoint = %+v, want the unsuffixed normalized endpoint", tm.createdEndpoint)
	}
	if err := p.updateEndpoint(context.Background(), oldEndpoint, newEndpoint); err != nil {
		t.Fatal(err)
	}
	if tm.updatedEndpoint == nil || tm.updatedEndpoint.EndpointName != "192-0-2-1" {
		t.Fatalf("updated endpoint = %+v, want the unsuffixed normalized endpoint", tm.updatedEndpoint)
	}
	if tm.updatedEndpoint.Target != newEndpoint.DNSName {
		t.Fatalf("A-record update target = %q, want DNS name %q", tm.updatedEndpoint.Target, newEndpoint.DNSName)
	}

	if err := p.deleteEndpoint(context.Background(), newEndpoint); err != nil {
		t.Fatal(err)
	}
	if len(tm.deletedEndpoints) != 1 || tm.deletedEndpoints[0] != "192-0-2-1" {
		t.Fatalf("deleted endpoint names = %v, want [192-0-2-1]", tm.deletedEndpoints)
	}
}

func TestVanityHostnameMigration(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(fmt.Sprintf("explicit=%t", explicit), func(t *testing.T) {
			p, tm, dns := testProvider()
			oldEndpoint, newEndpoint := configuredEndpoint(), configuredEndpoint()
			newEndpoint.ProviderSpecific = append(newEndpoint.ProviderSpecific,
				ProviderSpecificProperty{Name: annotations.AnnotationHostname, Value: "new.example.com"})
			if explicit {
				property := ProviderSpecificProperty{Name: annotations.AnnotationProfileName, Value: "shared-profile"}
				oldEndpoint.ProviderSpecific = append(oldEndpoint.ProviderSpecific, property)
				newEndpoint.ProviderSpecific = append(newEndpoint.ProviderSpecific, property)
			}
			p.stateManager.SetProfile("app.example.com", tm.profile)
			dns.deleteErr = errors.New("delete failed")
			if err := p.updateEndpoint(context.Background(), oldEndpoint, newEndpoint); err == nil {
				t.Fatal("expected old vanity cleanup failure")
			}
			dns.deleteErr = nil
			if err := p.updateEndpoint(context.Background(), oldEndpoint, newEndpoint); err != nil {
				t.Fatalf("migration retry failed: %v", err)
			}
			if len(dns.created) == 0 || dns.created[len(dns.created)-1] != "new-example-com-tm-cname" {
				t.Fatalf("new vanity not reconciled: %v", dns.created)
			}
			if len(dns.deleted) != 2 || dns.deleted[1] != "app-example-com-tm-cname" {
				t.Fatalf("old vanity not retried: %v", dns.deleted)
			}
			if _, ok := p.stateManager.GetProfile("app.example.com"); ok {
				t.Fatal("old state remains")
			}
			if _, ok := p.stateManager.GetProfile("new.example.com"); !ok {
				t.Fatal("new state missing")
			}
			if explicit {
				if tm.updatedProfile == nil || tm.updatedProfile.ProfileName != "shared-profile" ||
					tm.updatedProfile.Tags["hostname"] != "new.example.com" {
					t.Fatalf("incorrect in-place profile update: %+v", tm.updatedProfile)
				}
			} else if tm.createdProfile == nil || tm.createdProfile.ProfileName != "new-example-com-tm" {
				t.Fatalf("replacement profile not created: %+v", tm.createdProfile)
			}
		})
	}
}

func TestVanityMigrationDoesNotRemoveOldRecordWhenNewRecordFails(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		p, _, dns := testProvider()
		oldEndpoint, newEndpoint := configuredEndpoint(), configuredEndpoint()
		newEndpoint.ProviderSpecific = append(newEndpoint.ProviderSpecific,
			ProviderSpecificProperty{Name: annotations.AnnotationHostname, Value: "new.example.com"})
		if explicit {
			property := ProviderSpecificProperty{Name: annotations.AnnotationProfileName, Value: "shared-profile"}
			oldEndpoint.ProviderSpecific = append(oldEndpoint.ProviderSpecific, property)
			newEndpoint.ProviderSpecific = append(newEndpoint.ProviderSpecific, property)
		}
		dns.createErr = errors.New("create failed")
		if err := p.updateEndpoint(context.Background(), oldEndpoint, newEndpoint); err == nil {
			t.Fatal("expected new vanity creation failure")
		}
		if len(dns.deleted) != 0 {
			t.Fatalf("deleted old vanity on failure: %v", dns.deleted)
		}
	}
}

func TestVanityMigrationPreservesOldSharedProfile(t *testing.T) {
	p, tm, dns := testProvider()
	tm.profile.Endpoints["west"] = &state.EndpointState{EndpointName: "west"}
	oldEndpoint, newEndpoint := configuredEndpoint(), configuredEndpoint()
	newEndpoint.ProviderSpecific = append(newEndpoint.ProviderSpecific,
		ProviderSpecificProperty{Name: annotations.AnnotationHostname, Value: "new.example.com"})
	if err := p.updateEndpoint(context.Background(), oldEndpoint, newEndpoint); err != nil {
		t.Fatal(err)
	}
	if len(dns.deleted) != 0 {
		t.Fatalf("removed old vanity record while profile still has endpoints: %v", dns.deleted)
	}
	for _, call := range tm.calls {
		if call == "delete-profile" {
			t.Fatal("deleted shared old profile")
		}
	}
	if _, ok := p.stateManager.GetProfile("app.example.com"); !ok {
		t.Fatal("old shared profile missing from state")
	}
}

func TestVanityMigrationRequiresReplacementFQDN(t *testing.T) {
	p, tm, dns := testProvider()
	tm.profile.FQDN = ""
	oldEndpoint, newEndpoint := configuredEndpoint(), configuredEndpoint()
	newEndpoint.ProviderSpecific = append(newEndpoint.ProviderSpecific,
		ProviderSpecificProperty{Name: annotations.AnnotationHostname, Value: "new.example.com"})
	if err := p.updateEndpoint(context.Background(), oldEndpoint, newEndpoint); err == nil {
		t.Fatal("expected missing FQDN error")
	}
	if len(dns.deleted) != 0 || len(tm.deletedEndpoints) != 0 {
		t.Fatal("cleaned up old resources without a replacement FQDN")
	}
}

func TestDeleteRetriesVanityCleanupAfterProfileDeletion(t *testing.T) {
	p, tm, dns := testProvider()
	p.stateManager.SetProfile("app.example.com", tm.profile)
	dns.err = errors.New("DNS endpoint unavailable")

	endpoint := configuredEndpoint()
	if err := p.deleteEndpoint(context.Background(), endpoint); err == nil {
		t.Fatal("expected the DNSEndpoint deletion error")
	}
	if len(tm.deletedProfileNames) != 2 || tm.deletedProfileNames[0] != "app-example-com-tm" ||
		tm.deletedProfileNames[1] != "app-example-com-tm" {
		t.Fatalf("deletion used unexpected profile names: %v", tm.deletedProfileNames)
	}
	if _, ok := p.stateManager.GetProfile("app.example.com"); ok {
		t.Fatal("deleted profile remained in state")
	}

	tm.stateErr = &azcore.ResponseError{StatusCode: http.StatusNotFound}
	tm.calls = nil
	dns.err = nil
	if err := p.deleteEndpoint(context.Background(), endpoint); err != nil {
		t.Fatalf("retry after profile deletion failed: %v", err)
	}
	for _, call := range tm.calls {
		if call == "delete-profile" {
			t.Fatal("retried deletion of a profile that is already absent")
		}
	}
	if len(dns.deleted) != 2 || dns.deleted[0] != "app-example-com-tm-cname" ||
		dns.deleted[1] != "app-example-com-tm-cname" {
		t.Fatalf("vanity DNSEndpoint was not retried: %v", dns.deleted)
	}
}

func TestRecordsAndAdjustment(t *testing.T) {
	p, tm, _ := testProvider()
	tm.profiles = []*state.ProfileState{
		{Hostname: "app.example.com", FQDN: "app.trafficmanager.net", Endpoints: map[string]*state.EndpointState{}},
		{Hostname: "outside.invalid", FQDN: "other.trafficmanager.net"},
		{Hostname: "", FQDN: "missing.trafficmanager.net"},
	}
	records, err := p.Records(context.Background())
	if err != nil || len(records) != 1 || records[0].RecordType != "CNAME" {
		t.Fatalf("unexpected records: %+v, %v", records, err)
	}
	if adjusted := p.AdjustEndpoints(context.Background(), records); adjusted[0] != records[0] {
		t.Fatal("pass-through adjustment changed the endpoint")
	}
	tm.fail = "sync"
	if _, err := p.Records(context.Background()); err == nil {
		t.Fatal("sync failure was hidden")
	}
	filter := p.GetDomainFilter()
	filter.Include[0] = "changed.invalid"
	if p.domainFilter[0] != "example.com" {
		t.Fatal("filter must be a copy")
	}
}
