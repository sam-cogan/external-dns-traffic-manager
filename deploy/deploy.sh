#!/bin/bash

# Main deployment script for External DNS Traffic Manager webhook

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_DIR="$SCRIPT_DIR"

escape_sed_replacement() {
    printf '%s' "$1" | sed 's/[\\&|]/\\&/g'
}

render_manifest() {
    local subscription_id="$1"
    local traffic_manager_rg="$2"
    local dns_zone_rg="$3"
    local domain_filter="$4"
    local tenant_id="$5"
    local client_id="$6"
    local image="$7"

    sed \
        -e "s|__AZURE_SUBSCRIPTION_ID__|$(escape_sed_replacement "$subscription_id")|g" \
        -e "s|__TM_RESOURCE_GROUP__|$(escape_sed_replacement "$traffic_manager_rg")|g" \
        -e "s|__DNS_ZONE_RESOURCE_GROUP__|$(escape_sed_replacement "$dns_zone_rg")|g" \
        -e "s|__DOMAIN_FILTER__|$(escape_sed_replacement "$domain_filter")|g" \
        -e "s|__AZURE_TENANT_ID__|$(escape_sed_replacement "$tenant_id")|g" \
        -e "s|__AZURE_CLIENT_ID__|$(escape_sed_replacement "$client_id")|g" \
        -e "s|__CONTAINER_IMAGE__|$(escape_sed_replacement "$image")|g" \
        "$SCRIPT_DIR/../examples/single-cluster/external-dns.yaml"
}

if [[ "${1:-}" == "--render" ]]; then
    render_manifest \
        "${SUBSCRIPTION_ID:?SUBSCRIPTION_ID is required}" \
        "${TM_RG:?TM_RG is required}" \
        "${DNS_RG:?DNS_RG is required}" \
        "${DOMAIN_FILTER:?DOMAIN_FILTER is required}" \
        "${TENANT_ID:?TENANT_ID is required}" \
        "${CLIENT_ID:?CLIENT_ID is required}" \
        "${FULL_IMAGE:?FULL_IMAGE is required}"
    exit 0
fi

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

SUBSCRIPTION_ID="${AZURE_SUBSCRIPTION_ID:-}"

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

DNS_RG="${DNS_ZONE_RESOURCE_GROUP:-}"
if [ "$DNS_RG" = "N/A" ] || [ "$DNS_RG" = "your-dns-rg" ]; then
    DNS_RG=""
fi
read -p "Enter Azure DNS Zone Resource Group [$DNS_RG]: " INPUT_DNS_RG
DNS_RG="${INPUT_DNS_RG:-$DNS_RG}"

TENANT_ID="${AZURE_TENANT_ID:-}"
if [ -z "$TENANT_ID" ] && command -v az &> /dev/null; then
    TENANT_ID=$(az account show --subscription "$SUBSCRIPTION_ID" --query tenantId -o tsv 2>/dev/null || true)
fi
read -p "Enter Azure Tenant ID [$TENANT_ID]: " INPUT_TENANT_ID
TENANT_ID="${INPUT_TENANT_ID:-$TENANT_ID}"

CLIENT_ID="${AZURE_CLIENT_ID:-${AKS_IDENTITY_CLIENT_ID:-}}"
read -p "Enter workload identity Client ID [$CLIENT_ID]: " INPUT_CLIENT_ID
CLIENT_ID="${INPUT_CLIENT_ID:-$CLIENT_ID}"

read -p "Enter container registry (e.g., myregistry.azurecr.io): " INPUT_REGISTRY
REGISTRY="${INPUT_REGISTRY}"

read -p "Enter domain filter (optional, e.g., example.com): " DOMAIN_FILTER
DOMAIN_FILTER="${DOMAIN_FILTER:-example.com}"

if [ -z "$SUBSCRIPTION_ID" ] || [ -z "$TM_RG" ] || [ -z "$DNS_RG" ] ||
    [ -z "$TENANT_ID" ] || [ -z "$CLIENT_ID" ]; then
    echo "Error: Subscription, Traffic Manager and DNS resource groups, tenant, and workload identity client IDs are required"
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
echo "  Azure DNS RG: $DNS_RG"
echo "  Azure Tenant ID: $TENANT_ID"
echo "  Workload Identity Client ID: $CLIENT_ID"
echo "  Container Registry: $REGISTRY"
echo "  Domain Filter: $DOMAIN_FILTER"
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

echo ""
echo "Step 2: Deploying Kubernetes resources..."

kubectl apply -f "$SCRIPT_DIR/../examples/single-cluster/rbac.yaml"
kubectl annotate serviceaccount external-dns -n external-dns \
    "azure.workload.identity/client-id=$CLIENT_ID" --overwrite
kubectl apply -f "$SCRIPT_DIR/kubernetes/dnsendpoint-crd.yaml"
kubectl wait --for=condition=Established --timeout=60s crd/dnsendpoints.externaldns.k8s.io

# Create temporary file with substituted values
TEMP_MANIFEST=$(mktemp)
render_manifest "$SUBSCRIPTION_ID" "$TM_RG" "$DNS_RG" "$DOMAIN_FILTER" "$TENANT_ID" "$CLIENT_ID" "$FULL_IMAGE" > "$TEMP_MANIFEST"

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
