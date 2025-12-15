#!/bin/bash
set -e

# Load configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/config.env"

echo "=========================================="
echo "Setting Up Multi-Cluster Infrastructure"
echo "=========================================="
echo "Subscription: ${SUBSCRIPTION_ID}"
echo "East Cluster: ${CLUSTER_EAST_NAME} (${CLUSTER_EAST_REGION})"
echo "West Cluster: ${CLUSTER_WEST_NAME} (${CLUSTER_WEST_REGION})"
echo ""

# Set subscription
echo "Setting Azure subscription..."
az account set --subscription "${SUBSCRIPTION_ID}"

# Create resource groups
echo ""
echo "Creating resource groups..."
for rg_name in "${SHARED_RG}" "${CLUSTER_EAST_RG}" "${CLUSTER_WEST_RG}" "${TM_PROFILE_RG}"; do
    if az group show --name "${rg_name}" &> /dev/null; then
        echo "✓ Resource group ${rg_name} already exists"
    else
        local location="${CLUSTER_EAST_REGION}"
        local tags="purpose=external-dns-traffic-manager environment=test"
        
        case "${rg_name}" in
            "${CLUSTER_EAST_RG}")
                tags="${tags} region=east"
                ;;
            "${CLUSTER_WEST_RG}")
                location="${CLUSTER_WEST_REGION}"
                tags="${tags} region=west"
                ;;
        esac
        
        az group create --name "${rg_name}" --location "${location}" --tags ${tags} > /dev/null
        echo "✓ Resource group ${rg_name} created"
    fi
done

# Check if ACR exists, create if not
echo ""
echo "Checking Azure Container Registry..."
if az acr show --name "${ACR_NAME}" --resource-group "${SHARED_RG}" &> /dev/null; then
    echo "✓ ACR ${ACR_NAME} already exists"
else
    echo "Creating ACR ${ACR_NAME}..."
    az acr create \
        --resource-group "${SHARED_RG}" \
        --name "${ACR_NAME}" \
        --sku Basic \
        --admin-enabled true
    echo "✓ ACR created"
fi

# Create AKS cluster - East
echo ""
echo "Creating AKS cluster in East US..."
if az aks show --name "${CLUSTER_EAST_NAME}" --resource-group "${CLUSTER_EAST_RG}" &> /dev/null; then
    echo "✓ AKS cluster ${CLUSTER_EAST_NAME} already exists"
    # Ensure ACR is attached
    echo "  Ensuring ACR access..."
    az aks update --name "${CLUSTER_EAST_NAME}" --resource-group "${CLUSTER_EAST_RG}" --attach-acr "${ACR_NAME}" &> /dev/null || true
else
    echo "  Creating cluster (this will take 10-15 minutes)..."
    az aks create \
        --resource-group "${CLUSTER_EAST_RG}" \
        --name "${CLUSTER_EAST_NAME}" \
        --location "${CLUSTER_EAST_REGION}" \
        --node-count "${CLUSTER_EAST_NODE_COUNT}" \
        --node-vm-size Standard_D2s_v3 \
        --network-plugin azure \
        --enable-managed-identity \
        --attach-acr "${ACR_NAME}" \
        --generate-ssh-keys \
        --no-ssh-key \
        --tags region=east environment=test \
        --yes
    echo "✓ AKS cluster ${CLUSTER_EAST_NAME} created"
fi

# Create AKS cluster - West
echo ""
echo "Creating AKS cluster in West US..."
if az aks show --name "${CLUSTER_WEST_NAME}" --resource-group "${CLUSTER_WEST_RG}" &> /dev/null; then
    echo "✓ AKS cluster ${CLUSTER_WEST_NAME} already exists"
    # Ensure ACR is attached
    echo "  Ensuring ACR access..."
    az aks update --name "${CLUSTER_WEST_NAME}" --resource-group "${CLUSTER_WEST_RG}" --attach-acr "${ACR_NAME}" &> /dev/null || true
else
    echo "  Creating cluster (this will take 10-15 minutes)..."
    az aks create \
        --resource-group "${CLUSTER_WEST_RG}" \
        --name "${CLUSTER_WEST_NAME}" \
        --location "${CLUSTER_WEST_REGION}" \
        --node-count "${CLUSTER_WEST_NODE_COUNT}" \
        --node-vm-size Standard_D2s_v3 \
        --network-plugin azure \
        --enable-managed-identity \
        --attach-acr "${ACR_NAME}" \
        --generate-ssh-keys \
        --no-ssh-key \
        --tags region=west environment=test \
        --yes
    echo "✓ AKS cluster ${CLUSTER_WEST_NAME} created"
fi

# Get AKS credentials
echo ""
echo "Getting AKS credentials..."
az aks get-credentials --resource-group "${CLUSTER_EAST_RG}" --name "${CLUSTER_EAST_NAME}" --context "aks-tm-east" --overwrite-existing
az aks get-credentials --resource-group "${CLUSTER_WEST_RG}" --name "${CLUSTER_WEST_NAME}" --context "aks-tm-west" --overwrite-existing

# Grant DNS Zone permissions to AKS managed identities
echo ""
echo "Granting DNS and Traffic Manager permissions to AKS clusters..."

# Get managed identity for East cluster
EAST_IDENTITY=$(az aks show --resource-group "${CLUSTER_EAST_RG}" --name "${CLUSTER_EAST_NAME}" --query "identityProfile.kubeletidentity.clientId" -o tsv)
echo "East cluster managed identity: ${EAST_IDENTITY}"

# Get managed identity for West cluster
WEST_IDENTITY=$(az aks show --resource-group "${CLUSTER_WEST_RG}" --name "${CLUSTER_WEST_NAME}" --query "identityProfile.kubeletidentity.clientId" -o tsv)
echo "West cluster managed identity: ${WEST_IDENTITY}"

# Get DNS zone resource ID
DNS_ZONE_ID=$(az network dns zone show --name "${DNS_ZONE_NAME}" --resource-group "${DNS_ZONE_RG}" --query "id" -o tsv)
echo "DNS Zone ID: ${DNS_ZONE_ID}"

# Assign DNS Zone Contributor role to both clusters
echo "Assigning DNS Zone Contributor role to East cluster..."
az role assignment create \
    --assignee "${EAST_IDENTITY}" \
    --role "DNS Zone Contributor" \
    --scope "${DNS_ZONE_ID}" \
    2>/dev/null || echo "  (Role assignment already exists)"

echo "Assigning DNS Zone Contributor role to West cluster..."
az role assignment create \
    --assignee "${WEST_IDENTITY}" \
    --role "DNS Zone Contributor" \
    --scope "${DNS_ZONE_ID}" \
    2>/dev/null || echo "  (Role assignment already exists)"

# Assign Contributor role on Traffic Manager resource group
echo "Assigning Contributor role on Traffic Manager RG to East cluster..."
az role assignment create \
    --assignee "${EAST_IDENTITY}" \
    --role "Contributor" \
    --resource-group "${TM_PROFILE_RG}" \
    2>/dev/null || echo "  (Role assignment already exists)"

echo "Assigning Contributor role on Traffic Manager RG to West cluster..."
az role assignment create \
    --assignee "${WEST_IDENTITY}" \
    --role "Contributor" \
    --resource-group "${TM_PROFILE_RG}" \
    2>/dev/null || echo "  (Role assignment already exists)"

# Assign AcrPull role on ACR
echo "Getting ACR resource ID..."
ACR_ID=$(az acr show --name "${ACR_NAME}" --query id -o tsv)

echo "Assigning AcrPull role to East cluster..."
az role assignment create \
    --assignee "${EAST_IDENTITY}" \
    --role "AcrPull" \
    --scope "${ACR_ID}" \
    2>/dev/null || echo "  (Role assignment already exists)"

echo "Assigning AcrPull role to West cluster..."
az role assignment create \
    --assignee "${WEST_IDENTITY}" \
    --role "AcrPull" \
    --scope "${ACR_ID}" \
    2>/dev/null || echo "  (Role assignment already exists)"

echo "✓ Permissions granted"

# Verify contexts
echo ""
echo "Verifying kubectl contexts..."
kubectl config use-context aks-tm-east
kubectl get nodes --context aks-tm-east
echo ""
kubectl config use-context aks-tm-west  
kubectl get nodes --context aks-tm-west

echo ""
echo "=========================================="
echo "✓ Infrastructure Setup Complete"
echo "=========================================="
echo ""
echo "Clusters created:"
echo "  - East: ${CLUSTER_EAST_NAME} (context: aks-tm-east)"
echo "  - West: ${CLUSTER_WEST_NAME} (context: aks-tm-west)"
echo ""
echo "Resource Groups:"
echo "  - Shared: ${SHARED_RG}"
echo "  - East: ${CLUSTER_EAST_RG}"
echo "  - West: ${CLUSTER_WEST_RG}"
echo "  - Traffic Manager: ${TM_PROFILE_RG}"
echo ""
echo "Next steps:"
echo "  1. ./deploy-webhooks.sh    # Deploy External DNS and webhook to both clusters"
echo "  2. ./deploy-apps.sh         # Deploy demo applications"
echo "  3. ./verify.sh              # Verify the deployment"
