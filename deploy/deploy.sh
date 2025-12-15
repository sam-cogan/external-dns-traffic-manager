#!/bin/bash

# Main deployment script for External DNS Traffic Manager webhook

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_DIR="$SCRIPT_DIR"

echo "========================================"
echo "External DNS Traffic Manager Deployment"
echo "========================================"
echo ""

# Check prerequisites
echo "Checking prerequisites..."

if ! command -v kubectl &> /dev/null; then
    echo "Error: kubectl is not installed"
    exit 1
fi

if ! command -v docker &> /dev/null; then
    echo "Error: docker is not installed"
    exit 1
fi

# Check if infrastructure config exists
if [ -f "$DEPLOY_DIR/azure/infrastructure.env" ]; then
    echo "Loading infrastructure configuration..."
    source "$DEPLOY_DIR/azure/infrastructure.env"
else
    echo "Warning: infrastructure.env not found. Have you run setup-infrastructure.sh?"
    echo ""
fi

# Check if connected to cluster
if ! kubectl cluster-info &> /dev/null; then
    echo "Error: Not connected to a Kubernetes cluster"
    echo "Run: az aks get-credentials --resource-group <rg> --name <cluster>"
    exit 1
fi

CURRENT_CONTEXT=$(kubectl config current-context)
echo "Current cluster: $CURRENT_CONTEXT"
echo ""

# Prompt for configuration
read -p "Enter your Azure Subscription ID [$AZURE_SUBSCRIPTION_ID]: " INPUT_SUB_ID
SUBSCRIPTION_ID="${INPUT_SUB_ID:-$AZURE_SUBSCRIPTION_ID}"

read -p "Enter Traffic Manager Resource Group [$TM_RESOURCE_GROUP]: " INPUT_TM_RG
TM_RG="${INPUT_TM_RG:-$TM_RESOURCE_GROUP}"

read -p "Enter container registry (e.g., myregistry.azurecr.io): " INPUT_REGISTRY
REGISTRY="${INPUT_REGISTRY}"

read -p "Enter domain filter (optional, e.g., example.com): " DOMAIN_FILTER

if [ -z "$SUBSCRIPTION_ID" ] || [ -z "$TM_RG" ]; then
    echo "Error: Subscription ID and Traffic Manager Resource Group are required"
    exit 1
fi

if [ -z "$REGISTRY" ]; then
    echo "Error: Container registry is required"
    exit 1
fi

echo ""
echo "Configuration:"
echo "  Subscription ID: $SUBSCRIPTION_ID"
echo "  Traffic Manager RG: $TM_RG"
echo "  Container Registry: $REGISTRY"
echo "  Domain Filter: ${DOMAIN_FILTER:-none}"
echo ""

read -p "Continue with deployment? (y/n): " CONFIRM
if [ "$CONFIRM" != "y" ]; then
    echo "Deployment cancelled"
    exit 0
fi

# Build and push Docker image
echo ""
echo "Step 1: Building Docker image..."
IMAGE_TAG="${IMAGE_TAG:-latest}"
FULL_IMAGE="$REGISTRY/external-dns-traffic-manager-webhook:$IMAGE_TAG"

echo "Building: $FULL_IMAGE"
docker build -t "$FULL_IMAGE" "$SCRIPT_DIR/.."

echo ""
read -p "Push image to registry? (y/n): " PUSH
if [ "$PUSH" == "y" ]; then
    echo "Pushing image..."
    docker push "$FULL_IMAGE"
    echo "Image pushed successfully"
else
    echo "Skipping image push. Make sure the image is available in your cluster."
fi

# Update ConfigMap with actual values
echo ""
echo "Step 2: Deploying Kubernetes resources..."

# Create temporary file with substituted values
TEMP_MANIFEST=$(mktemp)
sed -e "s|YOUR_SUBSCRIPTION_ID|$SUBSCRIPTION_ID|g" \
    -e "s|externaldns-tm-profiles|$TM_RG|g" \
    -e "s|your-registry.azurecr.io/external-dns-traffic-manager-webhook:latest|$FULL_IMAGE|g" \
    -e "s|example.com|${DOMAIN_FILTER:-example.com}|g" \
    "$DEPLOY_DIR/kubernetes/external-dns.yaml" > "$TEMP_MANIFEST"

# Apply the manifest
kubectl apply -f "$TEMP_MANIFEST"
rm "$TEMP_MANIFEST"

echo ""
echo "Waiting for deployment to be ready..."
kubectl wait --for=condition=available --timeout=300s \
    deployment/external-dns -n external-dns || true

# Check pod status
echo ""
echo "Pod status:"
kubectl get pods -n external-dns

echo ""
echo "========================================"
echo "Deployment Complete!"
echo "========================================"
echo ""
echo "Next steps:"
echo "  1. Check logs: kubectl logs -n external-dns deployment/external-dns -c traffic-manager-webhook -f"
echo "  2. Deploy demo app: kubectl apply -f $DEPLOY_DIR/kubernetes/demo-app.yaml"
echo "  3. Monitor services: kubectl get svc -w"
echo ""
echo "To test the demo app:"
echo "  1. Wait for LoadBalancer IPs to be assigned"
echo "  2. Check Traffic Manager profile in Azure Portal"
echo "  3. Test DNS resolution: nslookup demo.example.com"
echo "  4. Access via browser: http://demo.example.com"
