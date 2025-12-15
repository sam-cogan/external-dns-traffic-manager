# Multi-Cluster Example

This example demonstrates deploying External DNS with the Traffic Manager webhook across multiple AKS clusters in different regions, sharing a single Traffic Manager profile.

## Overview

This setup demonstrates:
- Two AKS clusters in different Azure regions (East US and West US)
- Single shared Traffic Manager profile
- Weighted traffic distribution across regions
- Health-based automatic failover

## Architecture

```
                    Traffic Manager Profile
                         (Weighted)
                            │
              ┌─────────────┴─────────────┐
              │                           │
          Weight: 50                  Weight: 50
              │                           │
    ┌─────────▼────────┐       ┌─────────▼────────┐
    │  AKS East US     │       │  AKS West US     │
    │                  │       │                  │
    │  demo-app-east   │       │  demo-app-west   │
    │  External DNS    │       │  External DNS    │
    │  + TM Webhook    │       │  + TM Webhook    │
    └──────────────────┘       └──────────────────┘
```

## Prerequisites

- Two AKS clusters in different regions
- Azure DNS zone configured
- Traffic Manager resource group created
- Azure Workload Identity configured on both clusters
- kubectl contexts configured for both clusters

## Files

- `rbac.yaml` - RBAC permissions (apply to both clusters)
- `external-dns-east.yaml` - External DNS deployment for East cluster
- `external-dns-west.yaml` - External DNS deployment for West cluster
- `demo-app-east.yaml` - Demo app for East cluster
- `demo-app-west.yaml` - Demo app for West cluster

## Deployment Steps

### 1. Deploy to East US Cluster

```bash
# Switch to East cluster
kubectl config use-context aks-east

# Deploy RBAC
kubectl apply -f rbac.yaml

# Update external-dns-east.yaml with your values, then deploy
kubectl apply -f external-dns-east.yaml

# Deploy demo app
kubectl apply -f demo-app-east.yaml

# Verify
kubectl get pods -n external-dns
kubectl get svc demo-app-east
```

### 2. Deploy to West US Cluster

```bash
# Switch to West cluster
kubectl config use-context aks-west

# Deploy RBAC
kubectl apply -f rbac.yaml

# Update external-dns-west.yaml with your values, then deploy
kubectl apply -f external-dns-west.yaml

# Deploy demo app
kubectl apply -f demo-app-west.yaml

# Verify
kubectl get pods -n external-dns
kubectl get svc demo-app-west
```

### 3. Verify Traffic Manager

```bash
# Check Traffic Manager profile
az network traffic-manager profile show \
  --name multi-demo-profile \
  --resource-group your-tm-rg \
  --output table

# List endpoints
az network traffic-manager endpoint list \
  --profile-name multi-demo-profile \
  --resource-group your-tm-rg \
  --type ExternalEndpoints \
  --output table

# Test DNS resolution
nslookup multi-demo.example.com
```

## How It Works

1. **Profile Sharing**: Both clusters specify the same profile name in annotations
2. **Endpoint Registration**: Each cluster adds its own endpoint to the shared profile
3. **Weight Distribution**: Traffic is split 50/50 between regions (configurable)
4. **Health Monitoring**: Traffic Manager monitors both endpoints
5. **Automatic Failover**: If one region fails health checks, all traffic goes to healthy region

## Traffic Distribution

To change traffic weights, update the annotations:

**90% to East, 10% to West:**
```yaml
# In demo-app-east.yaml
external-dns.alpha.kubernetes.io/webhook-traffic-manager-weight: "90"

# In demo-app-west.yaml
external-dns.alpha.kubernetes.io/webhook-traffic-manager-weight: "10"
```

Apply the changes:
```bash
kubectl --context aks-east apply -f demo-app-east.yaml
kubectl --context aks-west apply -f demo-app-west.yaml
```

## Testing Failover

### Simulate East Region Failure

```bash
# Scale down East deployment
kubectl --context aks-east scale deployment demo-app-east --replicas=0

# Wait for health check to fail (~30 seconds)
sleep 45

# All traffic now goes to West
curl multi-demo.example.com
# Should show "Region: West US"

# Restore East
kubectl --context aks-east scale deployment demo-app-east --replicas=2
```

## Monitoring

### Check Endpoint Health

```bash
# View endpoint status
az network traffic-manager endpoint show \
  --name demo-east \
  --profile-name multi-demo-profile \
  --resource-group your-tm-rg \
  --type ExternalEndpoints \
  --query endpointMonitorStatus
```

### Check Logs

```bash
# East cluster logs
kubectl --context aks-east logs -n external-dns \
  deployment/external-dns -c traffic-manager-webhook

# West cluster logs
kubectl --context aks-west logs -n external-dns \
  deployment/external-dns -c traffic-manager-webhook
```

## Cleanup

```bash
# East cluster
kubectl --context aks-east delete -f demo-app-east.yaml
kubectl --context aks-east delete -f external-dns-east.yaml
kubectl --context aks-east delete -f rbac.yaml

# West cluster
kubectl --context aks-west delete -f demo-app-west.yaml
kubectl --context aks-west delete -f external-dns-west.yaml
kubectl --context aks-west delete -f rbac.yaml

# Delete Traffic Manager profile (if no longer needed)
az network traffic-manager profile delete \
  --name multi-demo-profile \
  --resource-group your-tm-rg
```

## Advanced Scenarios

### Priority-Based Failover

Instead of weighted distribution, use priority routing:

```yaml
# Primary (East)
external-dns.alpha.kubernetes.io/webhook-traffic-manager-routing-method: "Priority"
external-dns.alpha.kubernetes.io/webhook-traffic-manager-priority: "1"

# Secondary (West)
external-dns.alpha.kubernetes.io/webhook-traffic-manager-routing-method: "Priority"
external-dns.alpha.kubernetes.io/webhook-traffic-manager-priority: "2"
```

### Performance-Based Routing

Route users to the closest region:

```yaml
external-dns.alpha.kubernetes.io/webhook-traffic-manager-routing-method: "Performance"
```

## Troubleshooting

**Issue: Endpoints not appearing in Traffic Manager**
- Verify both clusters can reach Traffic Manager API
- Check managed identity permissions
- Review webhook logs for errors

**Issue: Health checks failing**
- Verify LoadBalancer has external IP
- Check monitor path returns HTTP 200
- Ensure application is listening on monitor port

**Issue: DNS not resolving**
- Verify CNAME record created in Azure DNS
- Check domain filter includes your hostname
- Allow time for DNS propagation (up to 5 minutes)
