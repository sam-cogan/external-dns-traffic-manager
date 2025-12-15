package annotations

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/sam-cogan/external-dns-traffic-manager/pkg/trafficmanager"
)

// TrafficManagerConfig holds parsed Traffic Manager configuration from annotations
type TrafficManagerConfig struct {
	// Core configuration
	Enabled       bool
	ProfileName   string
	ResourceGroup string
	Hostname      string // Vanity hostname for Traffic Manager (e.g., demo.example.com)

	// Routing configuration
	RoutingMethod string
	Weight        int64
	Priority      int64

	// Endpoint configuration
	EndpointName     string
	EndpointLocation string
	EndpointStatus   string
	EndpointType     string

	// DNS configuration
	DNSTTL int64

	// Monitoring configuration
	MonitorProtocol      string
	MonitorPort          int64
	MonitorPath          string
	HealthChecksEnabled  bool
}

// ParseConfig parses Traffic Manager configuration from annotation labels
func ParseConfig(labels map[string]string) (*TrafficManagerConfig, error) {
	config := &TrafficManagerConfig{
		// Set defaults
		RoutingMethod:   DefaultRoutingMethod,
		Weight:          DefaultWeight,
		Priority:        DefaultPriority,
		DNSTTL:          DefaultDNSTTL,
		MonitorProtocol: DefaultMonitorProtocol,
		MonitorPort:     DefaultMonitorPort,
		MonitorPath:     DefaultMonitorPath,
		EndpointStatus:  DefaultEndpointStatus,
		EndpointType:    DefaultEndpointType,
	}

	// Check if Traffic Manager is enabled
	if enabled, ok := labels[AnnotationEnabled]; ok {
		config.Enabled = strings.ToLower(enabled) == "true"
	}

	if !config.Enabled {
		return config, nil
	}

	// Parse required fields
	config.ResourceGroup = labels[AnnotationResourceGroup]
	if config.ResourceGroup == "" {
		return nil, fmt.Errorf("annotation %s is required when Traffic Manager is enabled", AnnotationResourceGroup)
	}

	// Parse optional profile name
	if profileName, ok := labels[AnnotationProfileName]; ok && profileName != "" {
		config.ProfileName = profileName
	}

	// Parse optional vanity hostname
	if hostname, ok := labels[AnnotationHostname]; ok && hostname != "" {
		config.Hostname = hostname
	}

	// Parse routing method
	if routingMethod, ok := labels[AnnotationRoutingMethod]; ok && routingMethod != "" {
		config.RoutingMethod = routingMethod
	}

	// Parse weight
	if weight, ok := labels[AnnotationWeight]; ok && weight != "" {
		w, err := strconv.ParseInt(weight, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid weight value %q: %w", weight, err)
		}
		config.Weight = w
	}

	// Parse priority
	if priority, ok := labels[AnnotationPriority]; ok && priority != "" {
		p, err := strconv.ParseInt(priority, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid priority value %q: %w", priority, err)
		}
		config.Priority = p
	}

	// Parse endpoint name
	if endpointName, ok := labels[AnnotationEndpointName]; ok && endpointName != "" {
		config.EndpointName = endpointName
	}

	// Parse endpoint location (required for ExternalEndpoints)
	if location, ok := labels[AnnotationEndpointLocation]; ok && location != "" {
		config.EndpointLocation = location
	}

	// Parse endpoint status
	if status, ok := labels[AnnotationEndpointStatus]; ok && status != "" {
		config.EndpointStatus = status
	}

	// Parse DNS TTL
	if ttl, ok := labels[AnnotationDNSTTL]; ok && ttl != "" {
		t, err := strconv.ParseInt(ttl, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid DNS TTL value %q: %w", ttl, err)
		}
		config.DNSTTL = t
	}

	// Parse monitor protocol
	if protocol, ok := labels[AnnotationMonitorProtocol]; ok && protocol != "" {
		config.MonitorProtocol = protocol
	}

	// Parse monitor port
	if port, ok := labels[AnnotationMonitorPort]; ok && port != "" {
		p, err := strconv.ParseInt(port, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid monitor port value %q: %w", port, err)
		}
		config.MonitorPort = p
	}

	// Parse monitor path
	if path, ok := labels[AnnotationMonitorPath]; ok && path != "" {
		config.MonitorPath = path
	}

	// Parse health checks enabled
	if healthChecks, ok := labels[AnnotationHealthChecksEnabled]; ok && healthChecks != "" {
		enabled, err := strconv.ParseBool(healthChecks)
		if err != nil {
			return nil, fmt.Errorf("invalid health checks enabled value %q: %w", healthChecks, err)
		}
		config.HealthChecksEnabled = enabled
	}

	return config, nil
}

// ToProfileConfig converts TrafficManagerConfig to trafficmanager.ProfileConfig
func (c *TrafficManagerConfig) ToProfileConfig() *trafficmanager.ProfileConfig {
	config := trafficmanager.DefaultProfileConfig()
	
	if c.ProfileName != "" {
		config.ProfileName = c.ProfileName
	}
	config.ResourceGroup = c.ResourceGroup
	config.RoutingMethod = c.RoutingMethod
	config.DNSTTL = c.DNSTTL
	config.MonitorProtocol = c.MonitorProtocol
	config.MonitorPort = c.MonitorPort
	config.MonitorPath = c.MonitorPath
	config.HealthChecksEnabled = c.HealthChecksEnabled
	
	// Add managed-by tag
	if config.Tags == nil {
		config.Tags = make(map[string]string)
	}
	config.Tags["managedBy"] = "external-dns-traffic-manager-webhook"
	
	return config
}

// ToEndpointConfig converts TrafficManagerConfig to trafficmanager.EndpointConfig
func (c *TrafficManagerConfig) ToEndpointConfig(target string) *trafficmanager.EndpointConfig {
	config := trafficmanager.DefaultEndpointConfig()
	
	if c.EndpointName != "" {
		config.EndpointName = c.EndpointName
	}
	config.EndpointType = c.EndpointType
	config.Target = target
	config.Weight = c.Weight
	config.Priority = c.Priority
	config.Status = c.EndpointStatus
	config.Location = c.EndpointLocation
	
	return config
}
