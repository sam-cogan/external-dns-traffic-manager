#!/bin/bash

# Setup Azure DNS configuration for External DNS
# This creates the azure.json config for managed identity authentication

set -e

# Load infrastructure configuration
if [ -f "deploy/azure/infrastructure.env" ]; then
    source deploy/azure/infrastructure.env
else
    echo "Error: infrastructure.env not found. Run setup-infrastructure.sh first."
    exit 1
fi

echo "=================================="
echo "Setting up Azure DNS Configuration"
echo "=================================="
echo "Subscription ID: $AZURE_SUBSCRIPTION_ID"
echo "DNS Zone: $DNS_ZONE_NAME"
echo "DNS Zone RG: $DNS_ZONE_RESOURCE_GROUP"
echo "Managed Identity: $AKS_IDENTITY_CLIENT_ID"
echo ""

# Create Azure config secret for managed identity
echo "Creating azure-config secret..."
kubectl create secret generic azure-config \
  --from-literal=azure.json="{
  \"tenantId\": \"72f988bf-86f1-41af-91ab-2d7cd011db47\",
  \"subscriptionId\": \"$AZURE_SUBSCRIPTION_ID\",
  \"resourceGroup\": \"$DNS_ZONE_RESOURCE_GROUP\",
  \"useManagedIdentityExtension\": true,
  \"userAssignedIdentityID\": \"$AKS_IDENTITY_CLIENT_ID\"
}" \
  --namespace=external-dns \
  --dry-run=client -o yaml | kubectl apply -f -

echo "✓ Azure config secret created"
echo ""

# Update ConfigMap with DNS zone information
echo "Updating webhook-config ConfigMap..."
kubectl patch configmap webhook-config \
  --namespace=external-dns \
  --type merge \
  --patch "{\"data\":{
    \"AZURE_SUBSCRIPTION_ID\":\"$AZURE_SUBSCRIPTION_ID\",
    \"DOMAIN_FILTER\":\"$DNS_ZONE_NAME\",
    \"DNS_ZONE_NAME\":\"$DNS_ZONE_NAME\",
    \"DNS_ZONE_RESOURCE_GROUP\":\"$DNS_ZONE_RESOURCE_GROUP\"
  }}"

echo "✓ ConfigMap updated"
echo ""
echo "Configuration complete! External DNS can now manage Azure DNS records."
