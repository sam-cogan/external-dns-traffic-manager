package provider

// Endpoint represents a DNS endpoint from External DNS
// This matches the External DNS endpoint type used in webhook communication
type Endpoint struct {
	DNSName          string                     `json:"dnsName"`
	Targets          []string                   `json:"targets"`
	RecordType       string                     `json:"recordType"`
	SetIdentifier    string                     `json:"setIdentifier,omitempty"`
	RecordTTL        int64                      `json:"recordTTL,omitempty"`
	Labels           map[string]string          `json:"labels,omitempty"`
	ProviderSpecific []ProviderSpecificProperty `json:"providerSpecific,omitempty"`
}

// ProviderSpecificProperty holds provider-specific metadata
type ProviderSpecificProperty struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Changes represents DNS changes to be applied
type Changes struct {
	Create    []*Endpoint `json:"create"`
	UpdateOld []*Endpoint `json:"updateOld"`
	UpdateNew []*Endpoint `json:"updateNew"`
	Delete    []*Endpoint `json:"delete"`
}

// DomainFilter represents domain filtering for the provider
type DomainFilter struct {
	Include []string `json:"include,omitempty"`
	Exclude []string `json:"exclude,omitempty"`
}

// NegotiationResponse is the response for the negotiation endpoint
type NegotiationResponse struct {
	Version      string       `json:"version"`
	DomainFilter DomainFilter `json:"domainFilter"`
}

// RecordsResponse is the response for the GET /records endpoint
type RecordsResponse struct {
	Endpoints []*Endpoint `json:"endpoints"`
}

// AdjustEndpointsRequest is the request for POST /adjustendpoints
type AdjustEndpointsRequest struct {
	Endpoints []*Endpoint `json:"endpoints"`
}

// AdjustEndpointsResponse is the response for POST /adjustendpoints
type AdjustEndpointsResponse struct {
	Endpoints []*Endpoint `json:"endpoints"`
}

// HealthResponse is the response for the health check endpoint
type HealthResponse struct {
	Status string `json:"status"`
}
