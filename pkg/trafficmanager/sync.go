package trafficmanager

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/trafficmanager/armtrafficmanager"
	"github.com/samcogan/external-dns-traffic-manager/pkg/state"
	"go.uber.org/zap"
)

// SyncProfilesFromAzure queries all Traffic Manager profiles and returns them as state
func (c *Client) SyncProfilesFromAzure(ctx context.Context, resourceGroups []string) ([]*state.ProfileState, error) {
	c.logger.Info("Syncing Traffic Manager profiles from Azure",
		zap.Strings("resourceGroups", resourceGroups))

	var allProfiles []*state.ProfileState

	for _, rg := range resourceGroups {
		profiles, err := c.listProfilesInResourceGroup(ctx, rg)
		if err != nil {
			c.logger.Error("Failed to list profiles in resource group",
				zap.String("resourceGroup", rg),
				zap.Error(err))
			// Continue with other resource groups
			continue
		}
		allProfiles = append(allProfiles, profiles...)
	}

	c.logger.Info("Successfully synced profiles from Azure",
		zap.Int("profileCount", len(allProfiles)))

	return allProfiles, nil
}

// listProfilesInResourceGroup lists all profiles in a resource group with managed-by tag
func (c *Client) listProfilesInResourceGroup(ctx context.Context, resourceGroup string) ([]*state.ProfileState, error) {
	var profiles []*state.ProfileState

	pager := c.profilesClient.NewListByResourceGroupPager(resourceGroup, nil)

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get next page: %w", err)
		}

		for _, profile := range page.Value {
			// Check if this profile is managed by us
			if !isManagedByUs(profile) {
				continue
			}

			profileState := c.profileToState(resourceGroup, profile)
			profiles = append(profiles, profileState)
		}
	}

	return profiles, nil
}

// profileToState converts an Azure SDK profile to state.ProfileState
func (c *Client) profileToState(resourceGroup string, profile *armtrafficmanager.Profile) *state.ProfileState {
	profileState := &state.ProfileState{
		ProfileName:   *profile.Name,
		ResourceGroup: resourceGroup,
		Endpoints:     make(map[string]*state.EndpointState),
		Tags:          make(map[string]string),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		CachedAt:      time.Now(),
	}

	if profile.Properties != nil {
		if profile.Properties.DNSConfig != nil {
			if profile.Properties.DNSConfig.Fqdn != nil {
				profileState.FQDN = *profile.Properties.DNSConfig.Fqdn
			}
			if profile.Properties.DNSConfig.TTL != nil {
				profileState.DNSTTL = *profile.Properties.DNSConfig.TTL
			}
		}

		if profile.Properties.TrafficRoutingMethod != nil {
			profileState.RoutingMethod = string(*profile.Properties.TrafficRoutingMethod)
		}

		// Convert endpoints
		if profile.Properties.Endpoints != nil {
			for _, endpoint := range profile.Properties.Endpoints {
				if endpoint.Name != nil {
					endpointState := c.endpointToState(endpoint)
					profileState.Endpoints[*endpoint.Name] = endpointState
				}
			}
		}
	}

	// Copy tags
	if profile.Tags != nil {
		for k, v := range profile.Tags {
			if v != nil {
				profileState.Tags[k] = *v
			}
		}

		// Extract hostname from tags
		if hostname, ok := profileState.Tags["hostname"]; ok {
			profileState.Hostname = hostname
		}
	}

	return profileState
}

// endpointToState converts an Azure SDK endpoint to state.EndpointState
func (c *Client) endpointToState(endpoint *armtrafficmanager.Endpoint) *state.EndpointState {
	endpointState := &state.EndpointState{
		EndpointName: *endpoint.Name,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if endpoint.Type != nil {
		endpointState.EndpointType = *endpoint.Type
	}

	if endpoint.Properties != nil {
		if endpoint.Properties.Target != nil {
			endpointState.Target = *endpoint.Properties.Target
		}
		if endpoint.Properties.Weight != nil {
			endpointState.Weight = *endpoint.Properties.Weight
		}
		if endpoint.Properties.Priority != nil {
			endpointState.Priority = *endpoint.Properties.Priority
		}
		if endpoint.Properties.EndpointStatus != nil {
			endpointState.Status = string(*endpoint.Properties.EndpointStatus)
		}
		if endpoint.Properties.EndpointLocation != nil {
			endpointState.Location = *endpoint.Properties.EndpointLocation
		}
	}

	return endpointState
}

// isManagedByUs checks if a profile has the managed-by tag
func isManagedByUs(profile *armtrafficmanager.Profile) bool {
	if profile.Tags == nil {
		return false
	}

	managedBy, exists := profile.Tags["managedBy"]
	if !exists || managedBy == nil {
		return false
	}

	return *managedBy == "external-dns-traffic-manager-webhook"
}

// GetProfileState queries a single profile and returns its state
func (c *Client) GetProfileState(ctx context.Context, resourceGroup, profileName string) (*state.ProfileState, error) {
	resp, err := c.profilesClient.Get(ctx, resourceGroup, profileName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get profile: %w", err)
	}

	return c.profileToState(resourceGroup, &resp.Profile), nil
}
