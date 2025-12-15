package annotations

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseConfig_Disabled(t *testing.T) {
	labels := map[string]string{
		AnnotationEnabled: "false",
	}

	config, err := ParseConfig(labels)
	require.NoError(t, err)
	assert.False(t, config.Enabled)
}

func TestParseConfig_MinimalValid(t *testing.T) {
	labels := map[string]string{
		AnnotationEnabled:      "true",
		AnnotationResourceGroup: "my-rg",
	}

	config, err := ParseConfig(labels)
	require.NoError(t, err)
	assert.True(t, config.Enabled)
	assert.Equal(t, "my-rg", config.ResourceGroup)

	// Verify defaults
	assert.Equal(t, DefaultRoutingMethod, config.RoutingMethod)
	assert.Equal(t, DefaultWeight, config.Weight)
	assert.Equal(t, DefaultPriority, config.Priority)
	assert.Equal(t, DefaultDNSTTL, config.DNSTTL)
	assert.Equal(t, DefaultMonitorProtocol, config.MonitorProtocol)
	assert.Equal(t, DefaultMonitorPort, config.MonitorPort)
	assert.Equal(t, DefaultMonitorPath, config.MonitorPath)
	assert.Equal(t, DefaultEndpointStatus, config.EndpointStatus)
}

func TestParseConfig_MissingResourceGroup(t *testing.T) {
	labels := map[string]string{
		AnnotationEnabled: "true",
		// ResourceGroup missing
	}

	config, err := ParseConfig(labels)
	assert.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "required when Traffic Manager is enabled")
}

func TestParseConfig_AllFields(t *testing.T) {
	labels := map[string]string{
		AnnotationEnabled:         "true",
		AnnotationResourceGroup:   "prod-rg",
		AnnotationProfileName:     "custom-profile",
		AnnotationRoutingMethod:   "Priority",
		AnnotationWeight:          "150",
		AnnotationPriority:        "5",
		AnnotationEndpointName:    "east-endpoint",
		AnnotationEndpointLocation: "East US",
		AnnotationEndpointStatus:  "Disabled",
		AnnotationDNSTTL:          "60",
		AnnotationMonitorProtocol: "TCP",
		AnnotationMonitorPort:     "8080",
		AnnotationMonitorPath:     "/health",
	}

	config, err := ParseConfig(labels)
	require.NoError(t, err)
	assert.True(t, config.Enabled)
	assert.Equal(t, "prod-rg", config.ResourceGroup)
	assert.Equal(t, "custom-profile", config.ProfileName)
	assert.Equal(t, "Priority", config.RoutingMethod)
	assert.Equal(t, int64(150), config.Weight)
	assert.Equal(t, int64(5), config.Priority)
	assert.Equal(t, "east-endpoint", config.EndpointName)
	assert.Equal(t, "East US", config.EndpointLocation)
	assert.Equal(t, "Disabled", config.EndpointStatus)
	assert.Equal(t, int64(60), config.DNSTTL)
	assert.Equal(t, "TCP", config.MonitorProtocol)
	assert.Equal(t, int64(8080), config.MonitorPort)
	assert.Equal(t, "/health", config.MonitorPath)
}

func TestParseConfig_InvalidWeight(t *testing.T) {
	labels := map[string]string{
		AnnotationEnabled:      "true",
		AnnotationResourceGroup: "my-rg",
		AnnotationWeight:       "not-a-number",
	}

	config, err := ParseConfig(labels)
	assert.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "weight")
}

func TestParseConfig_InvalidPriority(t *testing.T) {
	labels := map[string]string{
		AnnotationEnabled:      "true",
		AnnotationResourceGroup: "my-rg",
		AnnotationPriority:     "invalid",
	}

	config, err := ParseConfig(labels)
	assert.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "priority")
}

func TestParseConfig_InvalidTTL(t *testing.T) {
	labels := map[string]string{
		AnnotationEnabled:      "true",
		AnnotationResourceGroup: "my-rg",
		AnnotationDNSTTL:       "abc",
	}

	config, err := ParseConfig(labels)
	assert.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "TTL")
}

func TestParseConfig_InvalidMonitorPort(t *testing.T) {
	labels := map[string]string{
		AnnotationEnabled:      "true",
		AnnotationResourceGroup: "my-rg",
		AnnotationMonitorPort:  "not-a-port",
	}

	config, err := ParseConfig(labels)
	assert.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "port")
}

func TestToProfileConfig(t *testing.T) {
	config := &TrafficManagerConfig{
		Enabled:         true,
		ProfileName:     "my-profile",
		ResourceGroup:   "my-rg",
		RoutingMethod:   "Weighted",
		DNSTTL:          45,
		MonitorProtocol: "HTTPS",
		MonitorPort:     443,
		MonitorPath:     "/healthz",
	}

	profileConfig := config.ToProfileConfig()

	assert.Equal(t, "my-profile", profileConfig.ProfileName)
	assert.Equal(t, "my-rg", profileConfig.ResourceGroup)
	assert.Equal(t, "Weighted", profileConfig.RoutingMethod)
	assert.Equal(t, int64(45), profileConfig.DNSTTL)
	assert.Equal(t, "HTTPS", profileConfig.MonitorProtocol)
	assert.Equal(t, int64(443), profileConfig.MonitorPort)
	assert.Equal(t, "/healthz", profileConfig.MonitorPath)
	assert.Equal(t, "global", profileConfig.Location)
	assert.Contains(t, profileConfig.Tags, "managedBy")
	assert.Equal(t, "external-dns-traffic-manager-webhook", profileConfig.Tags["managedBy"])
}

func TestToEndpointConfig(t *testing.T) {
	config := &TrafficManagerConfig{
		EndpointName:     "test-endpoint",
		EndpointLocation: "West US",
		EndpointType:     DefaultEndpointType,
		Weight:           200,
		Priority:         3,
		EndpointStatus:   "Enabled",
	}

	target := "20.30.40.50"
	endpointConfig := config.ToEndpointConfig(target)

	assert.Equal(t, "test-endpoint", endpointConfig.EndpointName)
	assert.Equal(t, target, endpointConfig.Target)
	assert.Equal(t, int64(200), endpointConfig.Weight)
	assert.Equal(t, int64(3), endpointConfig.Priority)
	assert.Equal(t, "Enabled", endpointConfig.Status)
	assert.Equal(t, "West US", endpointConfig.Location)
	assert.Equal(t, DefaultEndpointType, endpointConfig.EndpointType)
}

func TestAnnotationConstants(t *testing.T) {
	// Verify annotation prefix
	assert.Equal(t, "external-dns.alpha.kubernetes.io/webhook-", AnnotationPrefix)

	// Verify all annotations have correct prefix
	annotations := []string{
		AnnotationEnabled,
		AnnotationProfileName,
		AnnotationResourceGroup,
		AnnotationRoutingMethod,
		AnnotationWeight,
		AnnotationPriority,
		AnnotationEndpointName,
		AnnotationEndpointLocation,
		AnnotationEndpointStatus,
		AnnotationDNSTTL,
		AnnotationMonitorProtocol,
		AnnotationMonitorPort,
		AnnotationMonitorPath,
	}

	for _, annotation := range annotations {
		assert.Contains(t, annotation, AnnotationPrefix, "Annotation should have correct prefix")
	}
}

func TestDefaultValues(t *testing.T) {
	assert.Equal(t, "Weighted", DefaultRoutingMethod)
	assert.Equal(t, int64(100), DefaultWeight)
	assert.Equal(t, int64(1), DefaultPriority)
	assert.Equal(t, int64(30), DefaultDNSTTL)
	assert.Equal(t, "HTTPS", DefaultMonitorProtocol)
	assert.Equal(t, int64(443), DefaultMonitorPort)
	assert.Equal(t, "/", DefaultMonitorPath)
	assert.Equal(t, "Enabled", DefaultEndpointStatus)
	assert.Equal(t, "ExternalEndpoints", DefaultEndpointType)
}

func TestParseConfig_CaseInsensitiveEnabled(t *testing.T) {
	testCases := []struct {
		name     string
		value    string
		expected bool
	}{
		{"lowercase true", "true", true},
		{"uppercase TRUE", "TRUE", true},
		{"mixed case True", "True", true},
		{"false", "false", false},
		{"empty", "", false},
		{"invalid", "yes", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			labels := map[string]string{
				AnnotationEnabled:      tc.value,
				AnnotationResourceGroup: "test-rg",
			}

			config, err := ParseConfig(labels)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, config.Enabled)
		})
	}
}

func TestParseConfig_EmptyLabels(t *testing.T) {
	labels := map[string]string{}

	config, err := ParseConfig(labels)
	require.NoError(t, err)
	assert.False(t, config.Enabled)
}

func TestParseConfig_OnlyPartialFields(t *testing.T) {
	labels := map[string]string{
		AnnotationEnabled:      "true",
		AnnotationResourceGroup: "my-rg",
		AnnotationWeight:       "250",
		// Other fields use defaults
	}

	config, err := ParseConfig(labels)
	require.NoError(t, err)
	assert.True(t, config.Enabled)
	assert.Equal(t, "my-rg", config.ResourceGroup)
	assert.Equal(t, int64(250), config.Weight)
	// Defaults
	assert.Equal(t, DefaultRoutingMethod, config.RoutingMethod)
	assert.Equal(t, DefaultMonitorProtocol, config.MonitorProtocol)
}
