#!/bin/bash
set -e

# Load configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/config.env"

echo "=========================================="
echo "Verifying Multi-Cluster Deployment"
echo "=========================================="
echo ""

# Check services in both clusters
echo "1. Checking services..."
echo ""
echo "East Cluster:"
kubectl --context aks-tm-east get services -n default
echo ""
echo "West Cluster:"
kubectl --context aks-tm-west get services -n default
echo ""

# Get LoadBalancer IPs
EAST_IP=$(kubectl --context aks-tm-east get service ${APP_NAME} -n default -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
WEST_IP=$(kubectl --context aks-tm-west get service ${APP_NAME} -n default -o jsonpath='{.status.loadBalancer.ingress[0].ip}')

echo "LoadBalancer IPs:"
echo "  East: ${EAST_IP}"
echo "  West: ${WEST_IP}"
echo ""

# Check DNSEndpoints
echo "2. Checking DNSEndpoints..."
echo ""
echo "East Cluster:"
kubectl --context aks-tm-east get dnsendpoints -A
echo ""
echo "West Cluster:"
kubectl --context aks-tm-west get dnsendpoints -A
echo ""

# Check Traffic Manager profile
echo "3. Checking Traffic Manager Profile..."
echo ""
PROFILE_NAME="${APP_NAME}-${DNS_ZONE_NAME//./-}-tm"
az network traffic-manager profile show \
    --name "${PROFILE_NAME}" \
    --resource-group "${TM_PROFILE_RG}" \
    --query '{name:name,status:profileStatus,routing:trafficRoutingMethod,fqdn:dnsConfig.fqdn}' \
    -o table 2>/dev/null || echo "⚠ Traffic Manager profile not found yet (may still be creating)"

echo ""
echo "4. Checking Traffic Manager Endpoints..."
echo ""
az network traffic-manager endpoint list \
    --profile-name "${PROFILE_NAME}" \
    --resource-group "${TM_PROFILE_RG}" \
    --type ExternalEndpoints \
    --query '[].{name:name,target:target,priority:priority,status:endpointStatus,monitorStatus:endpointMonitorStatus}' \
    -o table 2>/dev/null || echo "⚠ Endpoints not found yet"

echo ""
echo "5. Checking Azure DNS Records..."
echo ""
echo "A Records for endpoints:"
az network dns record-set a list \
    --resource-group "${DNS_ZONE_RG}" \
    --zone-name "${DNS_ZONE_NAME}" \
    --query "[?contains(name, 'multi')].{name:name,ip:aRecords[0].ipv4Address,ttl:ttl}" \
    -o table

echo ""
echo "CNAME Record for Traffic Manager:"
az network dns record-set cname show \
    --resource-group "${DNS_ZONE_RG}" \
    --zone-name "${DNS_ZONE_NAME}" \
    --name "${APP_NAME}" \
    --query '{name:name,cname:cnameRecord.cname,ttl:ttl}' \
    -o table 2>/dev/null || echo "⚠ CNAME record not created yet"

echo ""
echo "6. Testing DNS Resolution..."
echo ""
echo "Vanity hostname (${APP_HOSTNAME}):"
nslookup ${APP_HOSTNAME} 8.8.8.8 || echo "⚠ DNS not propagated yet"

echo ""
echo "East endpoint (${APP_EAST_HOSTNAME}):"
nslookup ${APP_EAST_HOSTNAME} 8.8.8.8 || echo "⚠ DNS not propagated yet"

echo ""
echo "West endpoint (${APP_WEST_HOSTNAME}):"
nslookup ${APP_WEST_HOSTNAME} 8.8.8.8 || echo "⚠ DNS not propagated yet"

echo ""
echo "7. Checking Webhook Logs..."
echo ""
echo "East Cluster:"
kubectl --context aks-tm-east logs -n external-dns deployment/external-dns -c traffic-manager-webhook --tail=20 | grep -E "(Traffic Manager|DNSEndpoint|profile)" || true

echo ""
echo "West Cluster:"
kubectl --context aks-tm-west logs -n external-dns deployment/external-dns -c traffic-manager-webhook --tail=20 | grep -E "(Traffic Manager|DNSEndpoint|profile)" || true

echo ""
echo "=========================================="
echo "Verification Complete"
echo "=========================================="
echo ""
echo "Expected Results:"
echo "  ✓ Both services have LoadBalancer IPs"
echo "  ✓ One DNSEndpoint for vanity CNAME (in one cluster)"
echo "  ✓ Traffic Manager profile exists with 2 endpoints"
echo "  ✓ Azure DNS has A records for both endpoints"
echo "  ✓ Azure DNS has CNAME record pointing to Traffic Manager"
echo "  ✓ DNS resolution works for all hostnames"
echo ""
echo "Key Test: Profile should NOT be created twice!"
echo "  - First cluster creates the profile"
echo "  - Second cluster adds its endpoint to existing profile"
echo ""
