#!/bin/bash

# Build and push Docker image for the Traffic Manager webhook

set -e

# Load infrastructure config if available
if [ -f "deploy/azure/infrastructure.env" ]; then
    echo "Loading configuration from deploy/azure/infrastructure.env"
    source deploy/azure/infrastructure.env
fi

# Configuration
REGISTRY="${CONTAINER_REGISTRY:-your-registry.azurecr.io}"
IMAGE_NAME="${IMAGE_NAME:-external-dns-traffic-manager-webhook}"
IMAGE_TAG="${IMAGE_TAG:-latest}"
FULL_IMAGE="$REGISTRY/$IMAGE_NAME:$IMAGE_TAG"

echo "=================================="
echo "Building Docker Image"
echo "=================================="
echo "Registry: $REGISTRY"
echo "Image: $IMAGE_NAME"
echo "Tag: $IMAGE_TAG"
echo "Full Image: $FULL_IMAGE"
echo ""

# Build the image for linux/amd64 (required for AKS)
echo "Building image for linux/amd64..."
docker build --platform linux/amd64 -t "$FULL_IMAGE" .

echo ""
echo "✓ Image built successfully for linux/amd64!"

# If ACR is configured, offer to push
if [ -n "$ACR_NAME" ] && [ "$REGISTRY" != "your-registry.azurecr.io" ]; then
    echo ""
    echo "Pushing to Azure Container Registry: $ACR_NAME"
    
    # Login to ACR
    echo "Logging in to ACR..."
    az acr login --name "$ACR_NAME"
    
    # Push the image
    echo "Pushing image..."
    docker push "$FULL_IMAGE"
    
    echo ""
    echo "✓ Image pushed successfully to $FULL_IMAGE"
    
    # Verify the image manifest
    echo ""
    echo "Verifying image platform..."
    MANIFEST=$(az acr manifest show --name "$IMAGE_NAME:$IMAGE_TAG" --registry "$ACR_NAME" --query 'config.digest' -o tsv 2>/dev/null || echo "")
    
    if [ -n "$MANIFEST" ]; then
        echo "✓ Image manifest found in ACR"
        
        # Check architecture using docker manifest
        echo "Inspecting image architecture..."
        docker manifest inspect "$FULL_IMAGE" 2>/dev/null | grep -A 5 '"architecture"' | head -10 || echo "  (Manifest inspection not available)"
    fi
    
    # Verify AKS can access ACR
    echo ""
    echo "Verifying AKS can access ACR..."
    if [ -n "$AKS_CLUSTER_NAME" ] && [ -n "$RESOURCE_GROUP" ]; then
        ACR_ID=$(az acr show --name "$ACR_NAME" --resource-group "$RESOURCE_GROUP" --query id -o tsv)
        AKS_ACR_ACCESS=$(az aks check-acr --name "$AKS_CLUSTER_NAME" --resource-group "$RESOURCE_GROUP" --acr "$ACR_NAME" 2>&1)
        
        if echo "$AKS_ACR_ACCESS" | grep -q "successfully authenticated"; then
            echo "✓ AKS can authenticate to ACR"
        else
            echo "⚠ Warning: AKS might not have access to ACR"
            echo "  Re-running attach command..."
            az aks update --resource-group "$RESOURCE_GROUP" --name "$AKS_CLUSTER_NAME" --attach-acr "$ACR_NAME" --output none
            echo "✓ ACR access updated"
        fi
    fi
else
    echo ""
    echo "To push to Azure Container Registry:"
    echo "  1. az acr login --name <your-acr-name>"
    echo "  2. docker push $FULL_IMAGE"
    echo ""
    echo "Or run the infrastructure setup first:"
    echo "  ./deploy/azure/setup-infrastructure.sh"
fi

echo ""
echo "Next step:"
echo "  ./deploy/deploy.sh"
