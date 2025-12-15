package trafficmanager

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

// GetAzureCredential returns an Azure credential for authentication
// It uses DefaultAzureCredential which tries multiple authentication methods:
// 1. Environment variables (AZURE_CLIENT_ID, AZURE_TENANT_ID, AZURE_CLIENT_SECRET)
// 2. Managed Identity (when running in Azure)
// 3. Azure CLI (for local development)
func GetAzureCredential() (azcore.TokenCredential, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to obtain Azure credential: %w", err)
	}
	return cred, nil
}

// TestCredential tests if the credential can obtain a token
func TestCredential(ctx context.Context, cred azcore.TokenCredential) error {
	// Try to get a token to verify the credential works
	token, err := cred.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{"https://management.azure.com/.default"},
	})
	if err != nil {
		return fmt.Errorf("failed to obtain token: %w", err)
	}
	if token.Token == "" {
		return fmt.Errorf("obtained empty token")
	}
	return nil
}
