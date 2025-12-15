package annotations

import (
	"fmt"
)

// ValidateConfig validates a TrafficManagerConfig
func ValidateConfig(config *TrafficManagerConfig) error {
	if !config.Enabled {
		return nil
	}

	// Validate required fields
	if config.ResourceGroup == "" {
		return fmt.Errorf("resource group is required")
	}

	// Validate weight range (1-1000)
	if config.Weight < 1 || config.Weight > 1000 {
		return fmt.Errorf("weight must be between 1 and 1000, got %d", config.Weight)
	}

	// Validate priority range (1-1000)
	if config.Priority < 1 || config.Priority > 1000 {
		return fmt.Errorf("priority must be between 1 and 1000, got %d", config.Priority)
	}

	// Validate routing method
	validRoutingMethods := []string{"Weighted", "Priority", "Performance", "Geographic"}
	if !contains(validRoutingMethods, config.RoutingMethod) {
		return fmt.Errorf("invalid routing method %q, must be one of: %v", config.RoutingMethod, validRoutingMethods)
	}

	// Validate monitor protocol
	validProtocols := []string{"HTTP", "HTTPS", "TCP"}
	if !contains(validProtocols, config.MonitorProtocol) {
		return fmt.Errorf("invalid monitor protocol %q, must be one of: %v", config.MonitorProtocol, validProtocols)
	}

	// Validate endpoint status
	validStatuses := []string{"Enabled", "Disabled"}
	if !contains(validStatuses, config.EndpointStatus) {
		return fmt.Errorf("invalid endpoint status %q, must be one of: %v", config.EndpointStatus, validStatuses)
	}

	// Validate DNS TTL (minimum 30 seconds)
	if config.DNSTTL < 30 {
		return fmt.Errorf("DNS TTL must be at least 30 seconds, got %d", config.DNSTTL)
	}

	// Validate monitor port
	if config.MonitorPort < 1 || config.MonitorPort > 65535 {
		return fmt.Errorf("monitor port must be between 1 and 65535, got %d", config.MonitorPort)
	}

	// Validate endpoint location for ExternalEndpoints
	if config.EndpointType == "ExternalEndpoints" && config.EndpointLocation == "" {
		return fmt.Errorf("endpoint location is required for ExternalEndpoints")
	}

	return nil
}

// contains checks if a string slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
