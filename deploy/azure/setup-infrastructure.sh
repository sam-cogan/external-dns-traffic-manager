#!/bin/bash

# External DNS Traffic Manager - Azure Infrastructure Setup
# This script creates the required Azure resources for testing the webhook

set -e

# Configuration
RESOURCE_GROUP="${RESOURCE_GROUP:-externaldns-tm-demo}"
LOCATION="${LOCATION:-eastus}"
AKS_CLUSTER_NAME="${AKS_CLUSTER_NAME:-externaldns-aks}"
AKS_NODE_COUNT="${AKS_NODE_COUNT:-2}"
AKS_NODE_SIZE="${AKS_NODE_SIZE:-Standard_B2s}"
TM_RESOURCE_GROUP="${TM_RESOURCE_GROUP:-externaldns-tm-profiles}"
ACR_NAME="${ACR_NAME:-externaldnstm$RANDOM}"

# ACR names must be alphanumeric and globally unique
ACR_NAME=$(echo "$ACR_NAME" | tr -dc '[:alnum:]' | tr '[:upper:]' '[:lower:]')

echo "=================================="
echo "External DNS Traffic Manager Setup"
echo "=================================="
echo "Resource Group: $RESOURCE_GROUP"
echo "Location: $LOCATION"
echo "AKS Cluster: $AKS_CLUSTER_NAME"
echo "ACR Name: $ACR_NAME"
echo "Traffic Manager RG: $TM_RESOURCE_GROUP"
echo ""

# Check if Azure CLI is installed
if ! command -v az &> /dev/null; then
    echo "Error: Azure CLI is not installed. Please install it from https://docs.microsoft.com/en-us/cli/azure/install-azure-cli"
    exit 1
fi

# Check if logged in
echo "Checking Azure login status..."
if ! az account show &> /dev/null; then
    echo "Not logged in. Please run 'az login' first."
    exit 1
fi

SUBSCRIPTION_ID=$(az account show --query id -o tsv)
echo "Using subscription: $SUBSCRIPTION_ID"
echo ""

# Create resource group for AKS
echo "Checking/Creating resource group: $RESOURCE_GROUP"
if az group show --name "$RESOURCE_GROUP" &> /dev/null; then
    echo "  ✓ Resource group $RESOURCE_GROUP already exists"
else
    echo "  Creating resource group..."
    az group create \
        --name "$RESOURCE_GROUP" \
        --location "$LOCATION" \
        --output table
fi

# Create resource group for Traffic Manager profiles
echo ""
echo "Checking/Creating Traffic Manager resource group: $TM_RESOURCE_GROUP"
if az group show --name "$TM_RESOURCE_GROUP" &> /dev/null; then
    echo "  ✓ Resource group $TM_RESOURCE_GROUP already exists"
else
    echo "  Creating Traffic Manager resource group..."
    az group create \
        --name "$TM_RESOURCE_GROUP" \
        --location "$LOCATION" \
        --output table
fi

# Create Azure Container Registry
echo ""
echo "Checking/Creating Azure Container Registry: $ACR_NAME"
if az acr show --name "$ACR_NAME" --resource-group "$RESOURCE_GROUP" &> /dev/null; then
    echo "  ✓ ACR $ACR_NAME already exists"
else
    echo "  Creating ACR..."
    az acr create \
        --resource-group "$RESOURCE_GROUP" \
        --name "$ACR_NAME" \
        --sku Basic \
        --output table
fi

ACR_LOGIN_SERVER=$(az acr show --name "$ACR_NAME" --resource-group "$RESOURCE_GROUP" --query loginServer -o tsv)
echo "  ACR Login Server: $ACR_LOGIN_SERVER"

# Create AKS cluster
echo ""
echo "Checking/Creating AKS cluster: $AKS_CLUSTER_NAME"
if az aks show --resource-group "$RESOURCE_GROUP" --name "$AKS_CLUSTER_NAME" &> /dev/null; then
    echo "  ✓ AKS cluster $AKS_CLUSTER_NAME already exists"
else
    echo "  Creating AKS cluster (this may take 5-10 minutes)..."
    az aks create \
        --resource-group "$RESOURCE_GROUP" \
        --name "$AKS_CLUSTER_NAME" \
        --node-count "$AKS_NODE_COUNT" \
        --node-vm-size "$AKS_NODE_SIZE" \
        --enable-managed-identity \
        --generate-ssh-keys \
        --attach-acr "$ACR_NAME" \
        --output table
fi

# Get AKS credentials
echo ""
echo "Getting AKS credentials..."
az aks get-credentials \
    --resource-group "$RESOURCE_GROUP" \
    --name "$AKS_CLUSTER_NAME" \
    --overwrite-existing

# Get the AKS managed identity
echo ""
echo "Getting AKS managed identity..."
AKS_IDENTITY_ID=$(az aks show \
    --resource-group "$RESOURCE_GROUP" \
    --name "$AKS_CLUSTER_NAME" \
    --query identityProfile.kubeletidentity.clientId -o tsv)

echo "AKS Managed Identity Client ID: $AKS_IDENTITY_ID"

# Ensure AKS has access to ACR (in case cluster existed before)
echo ""
echo "Ensuring AKS has access to ACR..."
az aks update \
    --resource-group "$RESOURCE_GROUP" \
    --name "$AKS_CLUSTER_NAME" \
    --attach-acr "$ACR_NAME" \
    --output none
echo "  ✓ ACR access configured"

# Assign Traffic Manager Contributor role to AKS identity on TM resource group
echo ""
echo "Checking/Assigning Traffic Manager Contributor role..."
TM_SCOPE="/subscriptions/$SUBSCRIPTION_ID/resourceGroups/$TM_RESOURCE_GROUP"
if az role assignment list --assignee "$AKS_IDENTITY_ID" --role "Traffic Manager Contributor" --scope "$TM_SCOPE" --query '[0].id' -o tsv | grep -q .; then
    echo "  ✓ Traffic Manager Contributor role already assigned"
else
    echo "  Assigning Traffic Manager Contributor role..."
    az role assignment create \
        --assignee "$AKS_IDENTITY_ID" \
        --role "Traffic Manager Contributor" \
        --scope "$TM_SCOPE" \
        --output table
fi

# Also assign Reader role to allow listing profiles
echo ""
echo "Checking/Assigning Reader role..."
if az role assignment list --assignee "$AKS_IDENTITY_ID" --role "Reader" --scope "$TM_SCOPE" --query '[0].id' -o tsv | grep -q .; then
    echo "  ✓ Reader role already assigned"
else
    echo "  Assigning Reader role..."
    az role assignment create \
        --assignee "$AKS_IDENTITY_ID" \
        --role "Reader" \
        --scope "$TM_SCOPE" \
        --output table
fi

# Grant DNS Zone Contributor role for Azure DNS integration
DNS_ZONE_NAME="${DNS_ZONE_NAME:-lab-ms.samcogan.com}"
DNS_ZONE_RESOURCE_GROUP="${DNS_ZONE_RESOURCE_GROUP:-core}"
DNS_ZONE_SCOPE="/subscriptions/$SUBSCRIPTION_ID/resourceGroups/$DNS_ZONE_RESOURCE_GROUP/providers/Microsoft.Network/dnszones/$DNS_ZONE_NAME"

echo "Checking DNS Zone permissions..."
if az network dns zone show --name "$DNS_ZONE_NAME" --resource-group "$DNS_ZONE_RESOURCE_GROUP" &> /dev/null; then
    echo "  DNS Zone found: $DNS_ZONE_NAME"
    echo "  Granting DNS Zone Contributor role to managed identity..."
    if az role assignment create \
        --assignee "$AKS_IDENTITY_ID" \
        --role "DNS Zone Contributor" \
        --scope "$DNS_ZONE_SCOPE" &> /dev/null; then
        echo "  ✓ DNS Zone Contributor role assigned"
    else
        echo "  ℹ Role assignment already exists or completed"
    fi
else
    echo "  ⚠ DNS Zone not found: $DNS_ZONE_NAME in resource group $DNS_ZONE_RESOURCE_GROUP"
    echo "  Skipping DNS Zone permission setup"
    DNS_ZONE_NAME="example.com"
    DNS_ZONE_RESOURCE_GROUP="N/A"
fi

# Save configuration to file
CONFIG_FILE="deploy/azure/infrastructure.env"
echo ""
echo "Saving configuration to $CONFIG_FILE"
cat > "$CONFIG_FILE" <<EOF
# Azure Infrastructure Configuration
# Generated: $(date)

export AZURE_SUBSCRIPTION_ID="$SUBSCRIPTION_ID"
export RESOURCE_GROUP="$RESOURCE_GROUP"
export TM_RESOURCE_GROUP="$TM_RESOURCE_GROUP"
export AKS_CLUSTER_NAME="$AKS_CLUSTER_NAME"
export AKS_IDENTITY_CLIENT_ID="$AKS_IDENTITY_ID"
export LOCATION="$LOCATION"
export ACR_NAME="$ACR_NAME"
export ACR_LOGIN_SERVER="$ACR_LOGIN_SERVER"
export CONTAINER_REGISTRY="$ACR_LOGIN_SERVER"
export DNS_ZONE_NAME="$DNS_ZONE_NAME"
export DNS_ZONE_RESOURCE_GROUP="$DNS_ZONE_RESOURCE_GROUP"
EOF

echo ""
echo "=================================="
echo "Infrastructure Setup Complete!"
echo "=================================="
echo ""
echo "Resources created/verified:"
echo "  - Resource Group: $RESOURCE_GROUP"
echo "  - AKS Cluster: $AKS_CLUSTER_NAME"
echo "  - Container Registry: $ACR_NAME ($ACR_LOGIN_SERVER)"
echo "  - Traffic Manager RG: $TM_RESOURCE_GROUP"
echo "  - Managed Identity: $AKS_IDENTITY_ID"
echo ""
echo "Next steps:"
echo "  1. Build and push the webhook Docker image:"
echo "     ./deploy/azure/build-push.sh"
echo "  2. Deploy External DNS and webhook:"
echo "     ./deploy/deploy.sh"
echo "  3. Deploy demo application:"
echo "     kubectl apply -f deploy/kubernetes/demo-app.yaml"
echo ""
echo "Configuration saved to: $CONFIG_FILE"
echo "Source it with: source $CONFIG_FILE"
