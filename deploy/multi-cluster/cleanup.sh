#!/bin/bash
set -e

# Load configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/config.env"

echo "=========================================="
echo "Cleaning Up Multi-Cluster Resources"
echo "=========================================="
echo ""

read -p "This will delete all resources. Are you sure? (yes/no): " CONFIRM
if [ "${CONFIRM}" != "yes" ]; then
    echo "Cleanup cancelled"
    exit 0
fi

echo ""
echo "Deleting applications from clusters..."

# Delete from East
kubectl --context aks-tm-east delete deployment,service,configmap -l app=${APP_NAME} -n default --ignore-not-found=true 2>/dev/null || true
kubectl --context aks-tm-east delete dnsendpoints --all -n default --ignore-not-found=true 2>/dev/null || true

# Delete from West
kubectl --context aks-tm-west delete deployment,service,configmap -l app=${APP_NAME} -n default --ignore-not-found=true 2>/dev/null || true
kubectl --context aks-tm-west delete dnsendpoints --all -n default --ignore-not-found=true 2>/dev/null || true

echo "✓ Applications deleted"

echo ""
echo "Waiting for Traffic Manager cleanup (30 seconds)..."
sleep 30

echo ""
echo "Deleting Traffic Manager profiles..."
PROFILE_NAME="${APP_NAME}-${DNS_ZONE_NAME//./-}-tm"
az network traffic-manager profile delete \
    --name "${PROFILE_NAME}" \
    --resource-group "${TM_PROFILE_RG}" \
    --yes 2>/dev/null || echo "⚠ Profile already deleted or not found"

echo ""
echo "Deleting DNS records..."
az network dns record-set a delete \
    --resource-group "${DNS_ZONE_RG}" \
    --zone-name "${DNS_ZONE_NAME}" \
    --name "multi-east" \
    --yes 2>/dev/null || true

az network dns record-set a delete \
    --resource-group "${DNS_ZONE_RG}" \
    --zone-name "${DNS_ZONE_NAME}" \
    --name "multi-west" \
    --yes 2>/dev/null || true

az network dns record-set cname delete \
    --resource-group "${DNS_ZONE_RG}" \
    --zone-name "${DNS_ZONE_NAME}" \
    --name "multi" \
    --yes 2>/dev/null || true

echo "✓ DNS records deleted"

echo ""
read -p "Delete AKS clusters? (yes/no): " DELETE_CLUSTERS
if [ "${DELETE_CLUSTERS}" = "yes" ]; then
    echo "Deleting AKS clusters (this will take several minutes)..."
    
    az aks delete \
        --resource-group "${CLUSTER_EAST_RG}" \
        --name "${CLUSTER_EAST_NAME}" \
        --yes --no-wait
    
    az aks delete \
        --resource-group "${CLUSTER_WEST_RG}" \
        --name "${CLUSTER_WEST_NAME}" \
        --yes --no-wait
    
    echo "✓ AKS cluster deletion initiated"
fi

echo ""
read -p "Delete all resource groups? (yes/no): " DELETE_RGS
if [ "${DELETE_RGS}" = "yes" ]; then
    echo "Deleting resource groups..."
    
    az group delete --name "${CLUSTER_EAST_RG}" --yes --no-wait 2>/dev/null || true
    az group delete --name "${CLUSTER_WEST_RG}" --yes --no-wait 2>/dev/null || true
    az group delete --name "${TM_PROFILE_RG}" --yes --no-wait 2>/dev/null || true
    
    echo "✓ Resource group deletion initiated"
    echo ""
    echo "Note: ${SHARED_RG} was not deleted (contains ACR)"
    echo "      ${DNS_ZONE_RG} was not deleted (contains DNS zone)"
fi

echo ""
echo "=========================================="
echo "✓ Cleanup Complete"
echo "=========================================="
echo ""
echo "Monitor deletion progress:"
echo "  az group list --query \"[?tags.purpose=='external-dns-traffic-manager']\" -o table"
