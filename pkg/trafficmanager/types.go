package trafficmanager

import (
	"time"
)

// ProfileConfig holds configuration for creating a Traffic Manager profile
type ProfileConfig struct {
	ProfileName     string
	ResourceGroup   string
	Location        string            // Always "global" for Traffic Manager
	RoutingMethod   string            // Weighted, Priority, Performance, Geographic
	DNSTTL          int64             // DNS TTL in seconds
	MonitorProtocol      string            // HTTP, HTTPS, TCP
	MonitorPort          int64             // Port to monitor
	MonitorPath          string            // Path for HTTP/HTTPS monitoring
	HealthChecksEnabled  bool              // Enable or disable endpoint health checks
	Tags                 map[string]string // Azure resource tags
}

// ProfileState represents the current state of a Traffic Manager profile
type ProfileState struct {
	ProfileName   string
	ResourceGroup string
	Hostname      string // DNS hostname that points to this profile (e.g., demo.lab-ms.samcogan.com)
	FQDN          string // Traffic Manager FQDN (e.g., myapp-tm.trafficmanager.net)
	RoutingMethod string
	DNSTTL        int64
	Endpoints     map[string]*EndpointState
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// EndpointConfig holds configuration for creating a Traffic Manager endpoint
type EndpointConfig struct {
	EndpointName string
	EndpointType string // AzureEndpoints, ExternalEndpoints, NestedEndpoints
	Target       string // IP address or FQDN
	Weight       int64  // 1-1000 for weighted routing
	Priority     int64  // 1-1000 for priority routing
	Status       string // Enabled or Disabled
	Location     string // Azure region (required for ExternalEndpoints)
}

// EndpointState represents the current state of a Traffic Manager endpoint
type EndpointState struct {
	EndpointName string
	EndpointType string
	Target       string
	Weight       int64
	Priority     int64
	Status       string
	Location     string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// DefaultProfileConfig returns a ProfileConfig with sensible defaults
func DefaultProfileConfig() *ProfileConfig {
	return &ProfileConfig{
		Location:        "global",
		RoutingMethod:   "Weighted",
		DNSTTL:          30,
		MonitorProtocol:      "HTTPS",
		MonitorPort:          443,
		MonitorPath:          "/",
		HealthChecksEnabled:  true,
		Tags:                 make(map[string]string),
	}
}

// DefaultEndpointConfig returns an EndpointConfig with sensible defaults
func DefaultEndpointConfig() *EndpointConfig {
	return &EndpointConfig{
		EndpointType: "ExternalEndpoints",
		Weight:       100,
		Priority:     1,
		Status:       "Enabled",
	}
}
