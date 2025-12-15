package state

import (
	"time"
)

// ProfileState represents the current state of a Traffic Manager profile
type ProfileState struct {
	ProfileName   string
	ResourceGroup string
	Hostname      string                    // The DNS hostname this profile manages
	FQDN          string                    // Traffic Manager FQDN (e.g., myapp-tm.trafficmanager.net)
	RoutingMethod string                    // Weighted, Priority, Performance, Geographic
	DNSTTL        int64                     // DNS TTL in seconds
	Endpoints     map[string]*EndpointState // Map of endpoint name to endpoint state
	Tags          map[string]string         // Azure resource tags
	CreatedAt     time.Time
	UpdatedAt     time.Time
	CachedAt      time.Time // When this state was last cached
}

// EndpointState represents the current state of a Traffic Manager endpoint
type EndpointState struct {
	EndpointName string
	EndpointType string // AzureEndpoints, ExternalEndpoints, NestedEndpoints
	Target       string // IP address or FQDN
	Weight       int64  // 1-1000 for weighted routing
	Priority     int64  // 1-1000 for priority routing
	Status       string // Enabled or Disabled
	Location     string // Azure region
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Clone creates a deep copy of ProfileState
func (ps *ProfileState) Clone() *ProfileState {
	clone := &ProfileState{
		ProfileName:   ps.ProfileName,
		ResourceGroup: ps.ResourceGroup,
		Hostname:      ps.Hostname,
		FQDN:          ps.FQDN,
		RoutingMethod: ps.RoutingMethod,
		DNSTTL:        ps.DNSTTL,
		Endpoints:     make(map[string]*EndpointState),
		Tags:          make(map[string]string),
		CreatedAt:     ps.CreatedAt,
		UpdatedAt:     ps.UpdatedAt,
		CachedAt:      ps.CachedAt,
	}

	// Deep copy endpoints
	for k, v := range ps.Endpoints {
		clone.Endpoints[k] = v.Clone()
	}

	// Copy tags
	for k, v := range ps.Tags {
		clone.Tags[k] = v
	}

	return clone
}

// Clone creates a deep copy of EndpointState
func (es *EndpointState) Clone() *EndpointState {
	return &EndpointState{
		EndpointName: es.EndpointName,
		EndpointType: es.EndpointType,
		Target:       es.Target,
		Weight:       es.Weight,
		Priority:     es.Priority,
		Status:       es.Status,
		Location:     es.Location,
		CreatedAt:    es.CreatedAt,
		UpdatedAt:    es.UpdatedAt,
	}
}

// IsExpired checks if the cached state has expired
func (ps *ProfileState) IsExpired(ttl time.Duration) bool {
	if ps.CachedAt.IsZero() {
		return true
	}
	return time.Since(ps.CachedAt) > ttl
}
