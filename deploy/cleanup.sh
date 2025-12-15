#!/bin/bash

# Cleanup script for External DNS Traffic Manager demo
# This removes all Azure resources and Kubernetes deployments

set -e

# Load configuration if it exists
if [ -f "deploy/azure/infrastructure.env" ]; then
    source deploy/azure/infrastructure.env
    echo "Loaded configuration from infrastructure.env"
else
    echo "Warning: infrastructure.env not found. You'll need to provide resource names manually."
    read -p "Resource Group name: " RESOURCE_GROUP
    read -p "Traffic Manager Resource Group name: " TM_RESOURCE_GROUP
fi

echo "=================================="
echo "External DNS Traffic Manager Cleanup"
echo "=================================="
echo "This will delete:"
echo "  - Kubernetes deployments in external-dns namespace"
echo "  - Demo application deployments"
echo "  - Azure Resource Group: $RESOURCE_GROUP (AKS, ACR)"
echo "  - Azure Resource Group: $TM_RESOURCE_GROUP (Traffic Manager)"
echo ""
read -p "Are you sure you want to proceed? (yes/no): " CONFIRM

if [ "$CONFIRM" != "yes" ]; then
    echo "Cleanup cancelled."
    exit 0
fi

echo ""
echo "Starting cleanup..."

# Delete Kubernetes resources
echo ""
echo "Deleting Kubernetes resources..."
kubectl delete -f deploy/kubernetes/demo-app-crd.yaml --ignore-not-found=true || true
kubectl delete -f deploy/kubernetes/external-dns.yaml --ignore-not-found=true || true
kubectl delete -f deploy/kubernetes/dnsendpoint-crd.yaml --ignore-not-found=true || true
kubectl delete namespace external-dns --ignore-not-found=true || true
echo "✓ Kubernetes resources deleted"

# Delete Azure resource groups
if [ -n "$RESOURCE_GROUP" ]; then
    echo ""
    echo "Deleting Azure resource group: $RESOURCE_GROUP"
    az group delete --name "$RESOURCE_GROUP" --yes --no-wait
    echo "✓ Deletion initiated for $RESOURCE_GROUP (running in background)"
fi

if [ -n "$TM_RESOURCE_GROUP" ]; then
    echo ""
    echo "Deleting Traffic Manager resource group: $TM_RESOURCE_GROUP"
    az group delete --name "$TM_RESOURCE_GROUP" --yes --no-wait
    echo "✓ Deletion initiated for $TM_RESOURCE_GROUP (running in background)"
fi

echo ""
echo "=================================="
echo "Cleanup initiated successfully!"
echo "=================================="
echo ""
echo "Note: Azure resource groups are being deleted in the background."
echo "This may take several minutes to complete."
echo ""
echo "To check deletion status:"
echo "  az group show --name $RESOURCE_GROUP"
echo "  az group show --name $TM_RESOURCE_GROUP"
