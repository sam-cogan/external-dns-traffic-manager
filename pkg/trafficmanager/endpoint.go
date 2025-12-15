package trafficmanager

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/trafficmanager/armtrafficmanager"
	"go.uber.org/zap"
)

// CreateEndpoint creates a new Traffic Manager endpoint
func (c *Client) CreateEndpoint(ctx context.Context, resourceGroup, profileName string, config *EndpointConfig) (*EndpointState, error) {
	c.logger.Info("Creating Traffic Manager endpoint",
		zap.String("profileName", profileName),
		zap.String("endpointName", config.EndpointName),
		zap.String("target", config.Target),
		zap.Int64("weight", config.Weight))

	endpoint := armtrafficmanager.Endpoint{
		Properties: &armtrafficmanager.EndpointProperties{
			Target:         &config.Target,
			Weight:         &config.Weight,
			Priority:       &config.Priority,
			EndpointStatus: toEndpointStatus(config.Status),
		},
	}

	// Add location for ExternalEndpoints
	if config.EndpointType == "ExternalEndpoints" {
		endpoint.Properties.EndpointLocation = &config.Location
	}

	resp, err := c.endpointsClient.CreateOrUpdate(
		ctx,
		resourceGroup,
		profileName,
		armtrafficmanager.EndpointType(config.EndpointType),
		config.EndpointName,
		endpoint,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create endpoint: %w", err)
	}

	c.logger.Info("Successfully created Traffic Manager endpoint",
		zap.String("endpointName", config.EndpointName),
		zap.String("target", config.Target))

	return endpointResponseToState(&resp.Endpoint), nil
}

// GetEndpoint retrieves a Traffic Manager endpoint
func (c *Client) GetEndpoint(ctx context.Context, resourceGroup, profileName, endpointType, endpointName string) (*EndpointState, error) {
	c.logger.Debug("Getting Traffic Manager endpoint",
		zap.String("profileName", profileName),
		zap.String("endpointName", endpointName))

	resp, err := c.endpointsClient.Get(
		ctx,
		resourceGroup,
		profileName,
		armtrafficmanager.EndpointType(endpointType),
		endpointName,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get endpoint: %w", err)
	}

	return endpointResponseToState(&resp.Endpoint), nil
}

// UpdateEndpoint updates an existing Traffic Manager endpoint
func (c *Client) UpdateEndpoint(ctx context.Context, resourceGroup, profileName string, config *EndpointConfig) (*EndpointState, error) {
	c.logger.Info("Updating Traffic Manager endpoint",
		zap.String("profileName", profileName),
		zap.String("endpointName", config.EndpointName))

	endpoint := armtrafficmanager.Endpoint{
		Properties: &armtrafficmanager.EndpointProperties{
			Target:         &config.Target,
			Weight:         &config.Weight,
			Priority:       &config.Priority,
			EndpointStatus: toEndpointStatus(config.Status),
		},
	}

	if config.EndpointType == "ExternalEndpoints" && config.Location != "" {
		endpoint.Properties.EndpointLocation = &config.Location
	}

	resp, err := c.endpointsClient.CreateOrUpdate(
		ctx,
		resourceGroup,
		profileName,
		armtrafficmanager.EndpointType(config.EndpointType),
		config.EndpointName,
		endpoint,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update endpoint: %w", err)
	}

	c.logger.Info("Successfully updated Traffic Manager endpoint",
		zap.String("endpointName", config.EndpointName))

	return endpointResponseToState(&resp.Endpoint), nil
}

// UpdateEndpointWeight updates only the weight of an endpoint
func (c *Client) UpdateEndpointWeight(ctx context.Context, resourceGroup, profileName, endpointType, endpointName string, weight int64) error {
	c.logger.Info("Updating endpoint weight",
		zap.String("profileName", profileName),
		zap.String("endpointName", endpointName),
		zap.Int64("weight", weight))

	// Get current endpoint
	current, err := c.GetEndpoint(ctx, resourceGroup, profileName, endpointType, endpointName)
	if err != nil {
		return err
	}

	// Update only the weight
	endpoint := armtrafficmanager.Endpoint{
		Properties: &armtrafficmanager.EndpointProperties{
			Target:         &current.Target,
			Weight:         &weight,
			Priority:       &current.Priority,
			EndpointStatus: toEndpointStatus(current.Status),
		},
	}

	if current.Location != "" {
		endpoint.Properties.EndpointLocation = &current.Location
	}

	_, err = c.endpointsClient.CreateOrUpdate(
		ctx,
		resourceGroup,
		profileName,
		armtrafficmanager.EndpointType(endpointType),
		endpointName,
		endpoint,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to update endpoint weight: %w", err)
	}

	c.logger.Info("Successfully updated endpoint weight",
		zap.String("endpointName", endpointName),
		zap.Int64("weight", weight))

	return nil
}

// UpdateEndpointStatus updates only the status (Enabled/Disabled) of an endpoint
func (c *Client) UpdateEndpointStatus(ctx context.Context, resourceGroup, profileName, endpointType, endpointName, status string) error {
	c.logger.Info("Updating endpoint status",
		zap.String("profileName", profileName),
		zap.String("endpointName", endpointName),
		zap.String("status", status))

	// Get current endpoint
	current, err := c.GetEndpoint(ctx, resourceGroup, profileName, endpointType, endpointName)
	if err != nil {
		return err
	}

	// Update only the status
	endpoint := armtrafficmanager.Endpoint{
		Properties: &armtrafficmanager.EndpointProperties{
			Target:         &current.Target,
			Weight:         &current.Weight,
			Priority:       &current.Priority,
			EndpointStatus: toEndpointStatus(status),
		},
	}

	if current.Location != "" {
		endpoint.Properties.EndpointLocation = &current.Location
	}

	_, err = c.endpointsClient.CreateOrUpdate(
		ctx,
		resourceGroup,
		profileName,
		armtrafficmanager.EndpointType(endpointType),
		endpointName,
		endpoint,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to update endpoint status: %w", err)
	}

	c.logger.Info("Successfully updated endpoint status",
		zap.String("endpointName", endpointName),
		zap.String("status", status))

	return nil
}

// DeleteEndpoint deletes a Traffic Manager endpoint
func (c *Client) DeleteEndpoint(ctx context.Context, resourceGroup, profileName, endpointType, endpointName string) error {
	c.logger.Info("Deleting Traffic Manager endpoint",
		zap.String("profileName", profileName),
		zap.String("endpointName", endpointName))

	_, err := c.endpointsClient.Delete(
		ctx,
		resourceGroup,
		profileName,
		armtrafficmanager.EndpointType(endpointType),
		endpointName,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to delete endpoint: %w", err)
	}

	c.logger.Info("Successfully deleted Traffic Manager endpoint",
		zap.String("endpointName", endpointName))

	return nil
}

// endpointResponseToState converts an SDK endpoint response to EndpointState
func endpointResponseToState(endpoint *armtrafficmanager.Endpoint) *EndpointState {
	state := &EndpointState{
		EndpointType: string(*endpoint.Type),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if endpoint.Name != nil {
		state.EndpointName = *endpoint.Name
	}

	if endpoint.Properties != nil {
		if endpoint.Properties.Target != nil {
			state.Target = *endpoint.Properties.Target
		}
		if endpoint.Properties.Weight != nil {
			state.Weight = *endpoint.Properties.Weight
		}
		if endpoint.Properties.Priority != nil {
			state.Priority = *endpoint.Properties.Priority
		}
		if endpoint.Properties.EndpointStatus != nil {
			state.Status = string(*endpoint.Properties.EndpointStatus)
		}
		if endpoint.Properties.EndpointLocation != nil {
			state.Location = *endpoint.Properties.EndpointLocation
		}
	}

	return state
}

// toEndpointStatus converts a string status to SDK EndpointStatus
func toEndpointStatus(status string) *armtrafficmanager.EndpointStatus {
	s := armtrafficmanager.EndpointStatus(status)
	return &s
}
