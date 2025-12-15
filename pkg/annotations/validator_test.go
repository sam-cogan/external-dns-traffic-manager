package annotations

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateConfig_Disabled(t *testing.T) {
	config := &TrafficManagerConfig{
		Enabled: false,
	}

	err := ValidateConfig(config)
	assert.NoError(t, err, "Disabled config should not be validated")
}

func TestValidateConfig_Valid(t *testing.T) {
	config := &TrafficManagerConfig{
		Enabled:          true,
		ResourceGroup:    "my-rg",
		RoutingMethod:    "Weighted",
		Weight:           100,
		Priority:         1,
		DNSTTL:           30,
		MonitorProtocol:  "HTTPS",
		MonitorPort:      443,
		EndpointStatus:   "Enabled",
		EndpointType:     "ExternalEndpoints",
		EndpointLocation: "East US",
	}

	err := ValidateConfig(config)
	assert.NoError(t, err)
}

func TestValidateConfig_MissingResourceGroup(t *testing.T) {
	config := &TrafficManagerConfig{
		Enabled:       true,
		ResourceGroup: "",
	}

	err := ValidateConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "resource group")
}

func TestValidateConfig_WeightTooLow(t *testing.T) {
	config := &TrafficManagerConfig{
		Enabled:       true,
		ResourceGroup: "my-rg",
		Weight:        0,
	}

	err := ValidateConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "weight")
}

func TestValidateConfig_WeightTooHigh(t *testing.T) {
	config := &TrafficManagerConfig{
		Enabled:       true,
		ResourceGroup: "my-rg",
		Weight:        1001,
	}

	err := ValidateConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "weight")
}

func TestValidateConfig_PriorityTooLow(t *testing.T) {
	config := &TrafficManagerConfig{
		Enabled:       true,
		ResourceGroup: "my-rg",
		Weight:        100,
		Priority:      0,
	}

	err := ValidateConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "priority")
}

func TestValidateConfig_PriorityTooHigh(t *testing.T) {
	config := &TrafficManagerConfig{
		Enabled:       true,
		ResourceGroup: "my-rg",
		Weight:        100,
		Priority:      1001,
	}

	err := ValidateConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "priority")
}

func TestValidateConfig_InvalidRoutingMethod(t *testing.T) {
	config := &TrafficManagerConfig{
		Enabled:       true,
		ResourceGroup: "my-rg",
		Weight:        100,
		Priority:      1,
		RoutingMethod: "InvalidMethod",
	}

	err := ValidateConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "routing method")
}

func TestValidateConfig_ValidRoutingMethods(t *testing.T) {
	methods := []string{"Weighted", "Priority", "Performance", "Geographic"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			config := &TrafficManagerConfig{
				Enabled:          true,
				ResourceGroup:    "my-rg",
				Weight:           100,
				Priority:         1,
				DNSTTL:           30,
				RoutingMethod:    method,
				MonitorProtocol:  "HTTPS",
				MonitorPort:      443,
				EndpointStatus:   "Enabled",
				EndpointType:     "ExternalEndpoints",
				EndpointLocation: "East US",
			}

			err := ValidateConfig(config)
			assert.NoError(t, err)
		})
	}
}

func TestValidateConfig_InvalidMonitorProtocol(t *testing.T) {
	config := &TrafficManagerConfig{
		Enabled:         true,
		ResourceGroup:   "my-rg",
		Weight:          100,
		Priority:        1,
		DNSTTL:          30,
		RoutingMethod:   "Weighted",
		MonitorProtocol: "FTP",
	}

	err := ValidateConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "protocol")
}

func TestValidateConfig_ValidMonitorProtocols(t *testing.T) {
	protocols := []string{"HTTP", "HTTPS", "TCP"}

	for _, protocol := range protocols {
		t.Run(protocol, func(t *testing.T) {
			config := &TrafficManagerConfig{
				Enabled:          true,
				ResourceGroup:    "my-rg",
				Weight:           100,
				Priority:         1,
				DNSTTL:           30,
				RoutingMethod:    "Weighted",
				MonitorProtocol:  protocol,
				MonitorPort:      443,
				EndpointStatus:   "Enabled",
				EndpointType:     "ExternalEndpoints",
				EndpointLocation: "East US",
			}

			err := ValidateConfig(config)
			assert.NoError(t, err)
		})
	}
}

func TestValidateConfig_InvalidEndpointStatus(t *testing.T) {
	config := &TrafficManagerConfig{
		Enabled:          true,
		ResourceGroup:    "my-rg",
		Weight:           100,
		Priority:         1,
		DNSTTL:           30,
		RoutingMethod:    "Weighted",
		MonitorProtocol:  "HTTPS",
		MonitorPort:      443,
		EndpointStatus:   "Active",
		EndpointType:     "ExternalEndpoints",
		EndpointLocation: "East US",
	}

	err := ValidateConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status")
}

func TestValidateConfig_TTLTooLow(t *testing.T) {
	config := &TrafficManagerConfig{
		Enabled:          true,
		ResourceGroup:    "my-rg",
		Weight:           100,
		Priority:         1,
		DNSTTL:           29, // Below minimum of 30
		RoutingMethod:    "Weighted",
		MonitorProtocol:  "HTTPS",
		MonitorPort:      443,
		EndpointStatus:   "Enabled",
		EndpointType:     "ExternalEndpoints",
		EndpointLocation: "East US",
	}

	err := ValidateConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "TTL")
}

func TestValidateConfig_PortTooLow(t *testing.T) {
	config := &TrafficManagerConfig{
		Enabled:          true,
		ResourceGroup:    "my-rg",
		Weight:           100,
		Priority:         1,
		DNSTTL:           30,
		RoutingMethod:    "Weighted",
		MonitorProtocol:  "HTTPS",
		MonitorPort:      0,
		EndpointStatus:   "Enabled",
		EndpointType:     "ExternalEndpoints",
		EndpointLocation: "East US",
	}

	err := ValidateConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "port")
}

func TestValidateConfig_PortTooHigh(t *testing.T) {
	config := &TrafficManagerConfig{
		Enabled:          true,
		ResourceGroup:    "my-rg",
		Weight:           100,
		Priority:         1,
		DNSTTL:           30,
		RoutingMethod:    "Weighted",
		MonitorProtocol:  "HTTPS",
		MonitorPort:      65536,
		EndpointStatus:   "Enabled",
		EndpointType:     "ExternalEndpoints",
		EndpointLocation: "East US",
	}

	err := ValidateConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "port")
}

func TestValidateConfig_ExternalEndpointWithoutLocation(t *testing.T) {
	config := &TrafficManagerConfig{
		Enabled:          true,
		ResourceGroup:    "my-rg",
		Weight:           100,
		Priority:         1,
		DNSTTL:           30,
		RoutingMethod:    "Weighted",
		MonitorProtocol:  "HTTPS",
		MonitorPort:      443,
		EndpointStatus:   "Enabled",
		EndpointType:     "ExternalEndpoints",
		EndpointLocation: "", // Missing location
	}

	err := ValidateConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "location")
}

func TestValidateConfig_AzureEndpointWithoutLocation(t *testing.T) {
	// Azure endpoints don't require location
	config := &TrafficManagerConfig{
		Enabled:          true,
		ResourceGroup:    "my-rg",
		Weight:           100,
		Priority:         1,
		DNSTTL:           30,
		RoutingMethod:    "Weighted",
		MonitorProtocol:  "HTTPS",
		MonitorPort:      443,
		EndpointStatus:   "Enabled",
		EndpointType:     "AzureEndpoints",
		EndpointLocation: "",
	}

	err := ValidateConfig(config)
	assert.NoError(t, err)
}

func TestValidateConfig_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		config    *TrafficManagerConfig
		shouldErr bool
		errText   string
	}{
		{
			name: "Weight exactly 1 (boundary)",
			config: &TrafficManagerConfig{
				Enabled:          true,
				ResourceGroup:    "my-rg",
				Weight:           1,
				Priority:         1,
				DNSTTL:           30,
				RoutingMethod:    "Weighted",
				MonitorProtocol:  "HTTPS",
				MonitorPort:      443,
				EndpointStatus:   "Enabled",
				EndpointType:     "ExternalEndpoints",
				EndpointLocation: "East US",
			},
			shouldErr: false,
		},
		{
			name: "Weight exactly 1000 (boundary)",
			config: &TrafficManagerConfig{
				Enabled:          true,
				ResourceGroup:    "my-rg",
				Weight:           1000,
				Priority:         1,
				DNSTTL:           30,
				RoutingMethod:    "Weighted",
				MonitorProtocol:  "HTTPS",
				MonitorPort:      443,
				EndpointStatus:   "Enabled",
				EndpointType:     "ExternalEndpoints",
				EndpointLocation: "East US",
			},
			shouldErr: false,
		},
		{
			name: "Port 1 (boundary)",
			config: &TrafficManagerConfig{
				Enabled:          true,
				ResourceGroup:    "my-rg",
				Weight:           100,
				Priority:         1,
				DNSTTL:           30,
				RoutingMethod:    "Weighted",
				MonitorProtocol:  "HTTPS",
				MonitorPort:      1,
				EndpointStatus:   "Enabled",
				EndpointType:     "ExternalEndpoints",
				EndpointLocation: "East US",
			},
			shouldErr: false,
		},
		{
			name: "Port 65535 (boundary)",
			config: &TrafficManagerConfig{
				Enabled:          true,
				ResourceGroup:    "my-rg",
				Weight:           100,
				Priority:         1,
				DNSTTL:           30,
				RoutingMethod:    "Weighted",
				MonitorProtocol:  "HTTPS",
				MonitorPort:      65535,
				EndpointStatus:   "Enabled",
				EndpointType:     "ExternalEndpoints",
				EndpointLocation: "East US",
			},
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(tt.config)
			if tt.shouldErr {
				assert.Error(t, err)
				if tt.errText != "" {
					assert.Contains(t, err.Error(), tt.errText)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
