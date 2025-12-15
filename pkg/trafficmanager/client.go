package trafficmanager

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/trafficmanager/armtrafficmanager"
	"go.uber.org/zap"
)

// Client wraps the Azure Traffic Manager SDK clients
type Client struct {
	profilesClient  *armtrafficmanager.ProfilesClient
	endpointsClient *armtrafficmanager.EndpointsClient
	subscriptionID  string
	logger          *zap.Logger
}

// NewClient creates a new Traffic Manager client
func NewClient(subscriptionID string, credential azcore.TokenCredential, logger *zap.Logger) (*Client, error) {
	if subscriptionID == "" {
		return nil, fmt.Errorf("subscription ID is required")
	}

	profilesClient, err := armtrafficmanager.NewProfilesClient(subscriptionID, credential, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create profiles client: %w", err)
	}

	endpointsClient, err := armtrafficmanager.NewEndpointsClient(subscriptionID, credential, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create endpoints client: %w", err)
	}

	return &Client{
		profilesClient:  profilesClient,
		endpointsClient: endpointsClient,
		subscriptionID:  subscriptionID,
		logger:          logger,
	}, nil
}

// TestConnection tests connectivity to Azure Traffic Manager API
func (c *Client) TestConnection(ctx context.Context, resourceGroup string) error {
	c.logger.Info("Testing Traffic Manager API connectivity",
		zap.String("resourceGroup", resourceGroup))

	// Try to list profiles in the resource group
	pager := c.profilesClient.NewListByResourceGroupPager(resourceGroup, nil)
	_, err := pager.NextPage(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to Traffic Manager API: %w", err)
	}

	c.logger.Info("Successfully connected to Traffic Manager API")
	return nil
}
