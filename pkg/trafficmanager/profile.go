package trafficmanager

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/trafficmanager/armtrafficmanager"
	"go.uber.org/zap"
)

// CreateProfile creates a new Traffic Manager profile
func (c *Client) CreateProfile(ctx context.Context, config *ProfileConfig) (*ProfileState, error) {
	c.logger.Info("Creating Traffic Manager profile",
		zap.String("profileName", config.ProfileName),
		zap.String("resourceGroup", config.ResourceGroup),
		zap.String("routingMethod", config.RoutingMethod),
		zap.String("location", config.Location),
		zap.Int64("dnsttl", config.DNSTTL))

	// Convert routing method to SDK type
	routingMethod := armtrafficmanager.TrafficRoutingMethod(config.RoutingMethod)

	// Build profile properties
	profile := armtrafficmanager.Profile{
		Location: toStringPtr(config.Location),
		Properties: &armtrafficmanager.ProfileProperties{
			TrafficRoutingMethod: &routingMethod,
			DNSConfig: &armtrafficmanager.DNSConfig{
				RelativeName: &config.ProfileName,
				TTL:          &config.DNSTTL,
			},
			MonitorConfig: &armtrafficmanager.MonitorConfig{
				Protocol: toMonitorProtocol(config.MonitorProtocol),
				Port:     &config.MonitorPort,
				Path:     &config.MonitorPath,
			},
			ProfileStatus: toProfileStatus(getProfileStatus(config.HealthChecksEnabled)),
		},
		Tags: toStringMapPtr(config.Tags),
	}

	// Create the profile
	resp, err := c.profilesClient.CreateOrUpdate(
		ctx,
		config.ResourceGroup,
		config.ProfileName,
		profile,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create profile: %w", err)
	}

	c.logger.Info("Successfully created Traffic Manager profile",
		zap.String("profileName", config.ProfileName),
		zap.String("fqdn", *resp.Properties.DNSConfig.Fqdn))

	return profileResponseToState(config.ResourceGroup, &resp.Profile), nil
}

// GetProfile retrieves a Traffic Manager profile
func (c *Client) GetProfile(ctx context.Context, resourceGroup, profileName string) (*ProfileState, error) {
	c.logger.Debug("Getting Traffic Manager profile",
		zap.String("profileName", profileName),
		zap.String("resourceGroup", resourceGroup))

	resp, err := c.profilesClient.Get(ctx, resourceGroup, profileName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get profile: %w", err)
	}

	return profileResponseToState(resourceGroup, &resp.Profile), nil
}

// UpdateProfile updates an existing Traffic Manager profile
func (c *Client) UpdateProfile(ctx context.Context, config *ProfileConfig) (*ProfileState, error) {
	c.logger.Info("Updating Traffic Manager profile",
		zap.String("profileName", config.ProfileName),
		zap.String("resourceGroup", config.ResourceGroup))

	// Get existing profile first
	existing, err := c.GetProfile(ctx, config.ResourceGroup, config.ProfileName)
	if err != nil {
		return nil, fmt.Errorf("failed to get existing profile: %w", err)
	}

	// Update only changed fields
	routingMethod := armtrafficmanager.TrafficRoutingMethod(config.RoutingMethod)
	profile := armtrafficmanager.Profile{
		Location: toStringPtr(config.Location),
		Properties: &armtrafficmanager.ProfileProperties{
			TrafficRoutingMethod: &routingMethod,
			DNSConfig: &armtrafficmanager.DNSConfig{
				TTL: &config.DNSTTL,
			},
			MonitorConfig: &armtrafficmanager.MonitorConfig{
				Protocol: toMonitorProtocol(config.MonitorProtocol),
				Port:     &config.MonitorPort,
				Path:     &config.MonitorPath,
			},
			ProfileStatus: toProfileStatus(getProfileStatus(config.HealthChecksEnabled)),
		},
		Tags: toStringMapPtr(config.Tags),
	}

	resp, err := c.profilesClient.CreateOrUpdate(
		ctx,
		config.ResourceGroup,
		config.ProfileName,
		profile,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update profile: %w", err)
	}

	c.logger.Info("Successfully updated Traffic Manager profile",
		zap.String("profileName", config.ProfileName))

	state := profileResponseToState(config.ResourceGroup, &resp.Profile)
	// Preserve endpoints from existing state
	state.Endpoints = existing.Endpoints
	return state, nil
}

// DeleteProfile deletes a Traffic Manager profile
func (c *Client) DeleteProfile(ctx context.Context, resourceGroup, profileName string) error {
	c.logger.Info("Deleting Traffic Manager profile",
		zap.String("profileName", profileName),
		zap.String("resourceGroup", resourceGroup))

	_, err := c.profilesClient.Delete(ctx, resourceGroup, profileName, nil)
	if err != nil {
		return fmt.Errorf("failed to delete profile: %w", err)
	}

	c.logger.Info("Successfully deleted Traffic Manager profile",
		zap.String("profileName", profileName))

	return nil
}

// ListProfiles lists all Traffic Manager profiles in a resource group
func (c *Client) ListProfiles(ctx context.Context, resourceGroup string) ([]*ProfileState, error) {
	c.logger.Debug("Listing Traffic Manager profiles",
		zap.String("resourceGroup", resourceGroup))

	var profiles []*ProfileState
	pager := c.profilesClient.NewListByResourceGroupPager(resourceGroup, nil)

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list profiles: %w", err)
		}

		for _, profile := range page.Value {
			profiles = append(profiles, profileResponseToState(resourceGroup, profile))
		}
	}

	c.logger.Debug("Successfully listed Traffic Manager profiles",
		zap.Int("count", len(profiles)))

	return profiles, nil
}

// profileResponseToState converts an SDK profile response to ProfileState
func profileResponseToState(resourceGroup string, profile *armtrafficmanager.Profile) *ProfileState {
	state := &ProfileState{
		ProfileName:   *profile.Name,
		ResourceGroup: resourceGroup,
		Endpoints:     make(map[string]*EndpointState),
		CreatedAt:     time.Now(), // SDK doesn't provide created time
		UpdatedAt:     time.Now(),
	}

	if profile.Properties != nil {
		if profile.Properties.DNSConfig != nil && profile.Properties.DNSConfig.Fqdn != nil {
			state.FQDN = *profile.Properties.DNSConfig.Fqdn
		}
		if profile.Properties.DNSConfig != nil && profile.Properties.DNSConfig.TTL != nil {
			state.DNSTTL = *profile.Properties.DNSConfig.TTL
		}
		if profile.Properties.TrafficRoutingMethod != nil {
			state.RoutingMethod = string(*profile.Properties.TrafficRoutingMethod)
		}

		// Convert endpoints if present
		if profile.Properties.Endpoints != nil {
			for _, endpoint := range profile.Properties.Endpoints {
				if endpoint.Name != nil {
					endpointState := endpointResponseToState(endpoint)
					state.Endpoints[*endpoint.Name] = endpointState
				}
			}
		}
	}

	return state
}

// Helper functions for pointer conversions
func toStringPtr(s string) *string {
	return &s
}

func toStringMapPtr(m map[string]string) map[string]*string {
	if m == nil {
		return nil
	}
	result := make(map[string]*string)
	for k, v := range m {
		value := v
		result[k] = &value
	}
	return result
}

func toMonitorProtocol(protocol string) *armtrafficmanager.MonitorProtocol {
	p := armtrafficmanager.MonitorProtocol(protocol)
	return &p
}

func toProfileStatus(status string) *armtrafficmanager.ProfileStatus {
	s := armtrafficmanager.ProfileStatus(status)
	return &s
}

func getProfileStatus(healthChecksEnabled bool) string {
	if healthChecksEnabled {
		return "Enabled"
	}
	return "Disabled"
}
