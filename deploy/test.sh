#!/bin/bash

# Test script for External DNS Traffic Manager webhook

set -e

echo "========================================="
echo "External DNS Traffic Manager - Testing"
echo "========================================="
echo ""

# Check if Traffic Manager RG is set
if [ -z "$TM_RESOURCE_GROUP" ]; then
    read -p "Enter Traffic Manager Resource Group: " TM_RESOURCE_GROUP
fi

if [ -z "$TM_RESOURCE_GROUP" ]; then
    echo "Error: Traffic Manager Resource Group is required"
    exit 1
fi

# Get profile name
echo "Finding Traffic Manager profiles..."
PROFILES=$(az network traffic-manager profile list -g "$TM_RESOURCE_GROUP" --query '[].name' -o tsv)

if [ -z "$PROFILES" ]; then
    echo "No Traffic Manager profiles found in resource group: $TM_RESOURCE_GROUP"
    echo ""
    echo "Have you deployed the demo app?"
    echo "  kubectl apply -f deploy/kubernetes/demo-app.yaml"
    exit 1
fi

echo "Found profiles:"
echo "$PROFILES"
echo ""

# Select first profile
PROFILE_NAME=$(echo "$PROFILES" | head -n 1)
echo "Using profile: $PROFILE_NAME"
echo ""

# Show profile details
echo "Profile Details:"
echo "==============="
az network traffic-manager profile show \
    -g "$TM_RESOURCE_GROUP" \
    -n "$PROFILE_NAME" \
    --query '{Name:name,Status:profileStatus,RoutingMethod:trafficRoutingMethod,TTL:dnsConfig.ttl,FQDN:dnsConfig.fqdn}' \
    -o table

echo ""
echo "Endpoints:"
echo "=========="
az network traffic-manager endpoint list \
    -g "$TM_RESOURCE_GROUP" \
    --profile-name "$PROFILE_NAME" \
    --query '[].{Name:name,Type:type,Target:target,Weight:weight,Priority:priority,Status:endpointStatus}' \
    -o table

# Get Traffic Manager FQDN
TM_FQDN=$(az network traffic-manager profile show \
    -g "$TM_RESOURCE_GROUP" \
    -n "$PROFILE_NAME" \
    --query dnsConfig.fqdn -o tsv)

echo ""
echo "Traffic Manager FQDN: $TM_FQDN"
echo ""

# Test DNS resolution
echo "Testing DNS Resolution:"
echo "======================"
if command -v nslookup &> /dev/null; then
    nslookup "$TM_FQDN" || true
else
    echo "nslookup not available, skipping DNS test"
fi

echo ""

# Test HTTP endpoints
echo "Testing HTTP Endpoints (10 requests):"
echo "====================================="

if ! command -v curl &> /dev/null; then
    echo "curl not available, skipping HTTP test"
else
    EAST_COUNT=0
    WEST_COUNT=0
    
    for i in {1..10}; do
        RESPONSE=$(curl -s -m 5 "http://$TM_FQDN" 2>/dev/null || echo "TIMEOUT")
        
        if echo "$RESPONSE" | grep -q "EAST"; then
            echo "Request $i: EAST ✓"
            ((EAST_COUNT++))
        elif echo "$RESPONSE" | grep -q "WEST"; then
            echo "Request $i: WEST ✓"
            ((WEST_COUNT++))
        else
            echo "Request $i: ERROR or TIMEOUT"
        fi
        
        sleep 0.5
    done
    
    echo ""
    echo "Results:"
    echo "  East: $EAST_COUNT requests ($(( EAST_COUNT * 10 ))%)"
    echo "  West: $WEST_COUNT requests ($(( WEST_COUNT * 10 ))%)"
    echo ""
    
    if [ $EAST_COUNT -gt 0 ] && [ $WEST_COUNT -gt 0 ]; then
        echo "✓ Load balancing is working! Traffic split between regions."
    elif [ $EAST_COUNT -gt 0 ] || [ $WEST_COUNT -gt 0 ]; then
        echo "⚠ Only one region responding. Check endpoint health."
    else
        echo "✗ No successful responses. Check services and endpoints."
    fi
fi

echo ""
echo "Additional Commands:"
echo "===================="
echo "View webhook logs:"
echo "  kubectl logs -n external-dns deployment/external-dns -c traffic-manager-webhook -f"
echo ""
echo "View service annotations:"
echo "  kubectl get svc demo-app-east -o yaml | grep webhook-traffic-manager"
echo ""
echo "Check service status:"
echo "  kubectl get svc"
echo ""
echo "Test in browser:"
echo "  open http://$TM_FQDN"
