package provider

import (
	"strings"
)

// matchesDomainFilter checks if a hostname matches the configured domain filter
func (p *TrafficManagerProvider) matchesDomainFilter(hostname string) bool {
	// If no domain filter configured, allow all
	if len(p.domainFilter) == 0 {
		return true
	}

	// Check if hostname matches any of the filters
	for _, filter := range p.domainFilter {
		if matchesDomain(hostname, filter) {
			return true
		}
	}

	return false
}

// matchesDomain checks if a hostname matches a domain filter pattern
// Supports exact match and wildcard subdomain matching
func matchesDomain(hostname, filter string) bool {
	// Exact match
	if hostname == filter {
		return true
	}

	// Wildcard subdomain match (e.g., filter "example.com" matches "app.example.com")
	if strings.HasSuffix(hostname, "."+filter) {
		return true
	}

	// Check if filter has wildcard prefix
	if strings.HasPrefix(filter, "*.") {
		suffix := filter[2:] // Remove "*."
		if hostname == suffix || strings.HasSuffix(hostname, "."+suffix) {
			return true
		}
	}

	return false
}
