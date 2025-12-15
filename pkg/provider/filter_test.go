package provider

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatchesDomainFilter_NoFilter(t *testing.T) {
	p := &TrafficManagerProvider{
		domainFilter: []string{},
	}

	// With no filter, everything matches
	assert.True(t, p.matchesDomainFilter("app.example.com"))
	assert.True(t, p.matchesDomainFilter("anything.com"))
	assert.True(t, p.matchesDomainFilter("test.internal"))
}

func TestMatchesDomainFilter_ExactMatch(t *testing.T) {
	p := &TrafficManagerProvider{
		domainFilter: []string{"example.com"},
	}

	assert.True(t, p.matchesDomainFilter("example.com"))
	assert.False(t, p.matchesDomainFilter("other.com"))
}

func TestMatchesDomainFilter_SubdomainMatch(t *testing.T) {
	p := &TrafficManagerProvider{
		domainFilter: []string{"example.com"},
	}

	// Subdomains should match
	assert.True(t, p.matchesDomainFilter("app.example.com"))
	assert.True(t, p.matchesDomainFilter("api.example.com"))
	assert.True(t, p.matchesDomainFilter("deep.subdomain.example.com"))

	// Non-subdomains should not match
	assert.False(t, p.matchesDomainFilter("examplexcom"))
	assert.False(t, p.matchesDomainFilter("notexample.com"))
}

func TestMatchesDomainFilter_WildcardMatch(t *testing.T) {
	p := &TrafficManagerProvider{
		domainFilter: []string{"*.example.com"},
	}

	// Wildcard should match base domain and subdomains
	assert.True(t, p.matchesDomainFilter("example.com"))
	assert.True(t, p.matchesDomainFilter("app.example.com"))
	assert.True(t, p.matchesDomainFilter("api.example.com"))

	// Should not match other domains
	assert.False(t, p.matchesDomainFilter("other.com"))
}

func TestMatchesDomainFilter_MultipleFilters(t *testing.T) {
	p := &TrafficManagerProvider{
		domainFilter: []string{"example.com", "test.com", "*.internal"},
	}

	// Should match any of the filters
	assert.True(t, p.matchesDomainFilter("example.com"))
	assert.True(t, p.matchesDomainFilter("app.example.com"))
	assert.True(t, p.matchesDomainFilter("test.com"))
	assert.True(t, p.matchesDomainFilter("api.test.com"))
	assert.True(t, p.matchesDomainFilter("internal"))
	assert.True(t, p.matchesDomainFilter("app.internal"))

	// Should not match if none of the filters match
	assert.False(t, p.matchesDomainFilter("other.com"))
	assert.False(t, p.matchesDomainFilter("notincluded.net"))
}

func TestMatchesDomain_ExactMatch(t *testing.T) {
	assert.True(t, matchesDomain("example.com", "example.com"))
	assert.False(t, matchesDomain("example.com", "other.com"))
}

func TestMatchesDomain_SubdomainMatch(t *testing.T) {
	filter := "example.com"

	assert.True(t, matchesDomain("example.com", filter))
	assert.True(t, matchesDomain("app.example.com", filter))
	assert.True(t, matchesDomain("api.app.example.com", filter))
	assert.False(t, matchesDomain("examplexcom", filter))
	assert.False(t, matchesDomain("notexample.com", filter))
}

func TestMatchesDomain_WildcardPrefix(t *testing.T) {
	filter := "*.example.com"

	// Should match base domain
	assert.True(t, matchesDomain("example.com", filter))

	// Should match subdomains
	assert.True(t, matchesDomain("app.example.com", filter))
	assert.True(t, matchesDomain("api.example.com", filter))
	assert.True(t, matchesDomain("deep.sub.example.com", filter))

	// Should not match other domains
	assert.False(t, matchesDomain("other.com", filter))
	assert.False(t, matchesDomain("example.net", filter))
}

func TestMatchesDomain_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		filter   string
		expected bool
	}{
		{
			name:     "Empty hostname",
			hostname: "",
			filter:   "example.com",
			expected: false,
		},
		{
			name:     "Empty filter",
			hostname: "app.example.com",
			filter:   "",
			expected: false,
		},
		{
			name:     "Single word hostname and filter",
			hostname: "localhost",
			filter:   "localhost",
			expected: true,
		},
		{
			name:     "Wildcard only",
			hostname: "example.com",
			filter:   "*.",
			expected: false,
		},
		{
			name:     "Hostname ends with dot",
			hostname: "app.example.com.",
			filter:   "example.com",
			expected: false,
		},
		{
			name:     "Filter with multiple wildcards (not standard but handled)",
			hostname: "app.example.com",
			filter:   "*.*.com",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesDomain(tt.hostname, tt.filter)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMatchesDomainFilter_RealWorldScenarios(t *testing.T) {
	tests := []struct {
		name     string
		filters  []string
		hostname string
		expected bool
	}{
		{
			name:     "Production domain",
			filters:  []string{"prod.example.com"},
			hostname: "api.prod.example.com",
			expected: true,
		},
		{
			name:     "Multiple environments",
			filters:  []string{"prod.example.com", "staging.example.com", "dev.example.com"},
			hostname: "api.staging.example.com",
			expected: true,
		},
		{
			name:     "Wildcard for all environments",
			filters:  []string{"*.example.com"},
			hostname: "anything.example.com",
			expected: true,
		},
		{
			name:     "Multiple domains",
			filters:  []string{"example.com", "myapp.io", "company.net"},
			hostname: "api.myapp.io",
			expected: true,
		},
		{
			name:     "Does not match different TLD",
			filters:  []string{"example.com"},
			hostname: "example.net",
			expected: false,
		},
		{
			name:     "Case sensitive (lowercase filter, uppercase hostname)",
			filters:  []string{"example.com"},
			hostname: "APP.EXAMPLE.COM",
			expected: false, // Current implementation is case-sensitive
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &TrafficManagerProvider{
				domainFilter: tt.filters,
			}
			result := p.matchesDomainFilter(tt.hostname)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "app.example.com",
			expected: "app-example-com",
		},
		{
			input:    "my-app",
			expected: "my-app",
		},
		{
			input:    "app_service",
			expected: "app-service",
		},
		{
			input:    "app@service",
			expected: "app-service",
		},
		{
			input:    "123-app",
			expected: "123-app",
		},
		{
			input:    "UPPERCASE",
			expected: "UPPERCASE",
		},
		{
			input:    "mixed.Case_123@test",
			expected: "mixed-Case-123-test",
		},
		{
			input:    "special!@#$%chars",
			expected: "special-----chars",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateProfileName(t *testing.T) {
	tests := []struct {
		name     string
		dnsName  string
		expected string
	}{
		{
			name:     "Simple domain",
			dnsName:  "app.example.com",
			expected: "app-example-com-tm",
		},
		{
			name:     "Subdomain",
			dnsName:  "api.prod.example.com",
			expected: "api-prod-example-com-tm",
		},
		{
			name:     "With underscores",
			dnsName:  "my_app.example.com",
			expected: "my-app-example-com-tm",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateProfileName(tt.dnsName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateEndpointName(t *testing.T) {
	tests := []struct {
		name     string
		dnsName  string
		targets  []string
		expected string
	}{
		{
			name:     "With target IP",
			dnsName:  "app.example.com",
			targets:  []string{"20.30.40.50"},
			expected: "20-30-40-50",
		},
		{
			name:     "With target hostname",
			dnsName:  "app.example.com",
			targets:  []string{"backend.internal"},
			expected: "backend-internal",
		},
		{
			name:     "No targets",
			dnsName:  "app.example.com",
			targets:  []string{},
			expected: "app-example-com",
		},
		{
			name:     "Multiple targets (uses first)",
			dnsName:  "app.example.com",
			targets:  []string{"20.30.40.50", "20.30.40.51"},
			expected: "20-30-40-50",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateEndpointName(tt.dnsName, tt.targets)
			assert.Equal(t, tt.expected, result)
		})
	}
}
