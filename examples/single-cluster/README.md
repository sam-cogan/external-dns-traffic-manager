# Single Cluster Example

This example demonstrates deploying External DNS with the Traffic Manager webhook in a single AKS cluster.

## Overview

This setup includes:
- External DNS with Azure DNS provider
- External DNS with Traffic Manager webhook provider
- Demo application with Traffic Manager annotations

## Prerequisites

- AKS cluster running
- Azure DNS zone configured
- Traffic Manager resource group created
- Azure Workload Identity configured

## Files

- `rbac.yaml` - RBAC permissions for External DNS
- `external-dns.yaml` - External DNS deployment with webhook
- `demo-app.yaml` - Sample application with Traffic Manager annotations

## Quick Deploy

1. Update the configuration values in `external-dns.yaml`:
   - Replace `your-subscription-id`
   - Replace `your-dns-rg`
   - Replace `your-tm-rg`
   - Replace `example.com` with your domain
   - Replace container image registry

2. Deploy External DNS:
```bash
kubectl apply -f rbac.yaml
kubectl apply -f external-dns.yaml
```

3. Deploy the demo application:
```bash
kubectl apply -f demo-app.yaml
```

4. Verify deployment:
```bash
# Check pods
kubectl get pods -n external-dns

# Check logs
kubectl logs -n external-dns deployment/external-dns -c traffic-manager-webhook

# Verify Service has external IP
kubectl get svc demo-app

# Check Traffic Manager profile created
az network traffic-manager profile list \
  --resource-group your-tm-rg \
  --output table
```

## How It Works

1. The demo app Service has Traffic Manager annotations
2. External DNS detects the Service
3. The webhook provider creates a Traffic Manager profile and endpoint
4. External DNS creates A and CNAME records in Azure DNS
5. Traffic is routed through Traffic Manager

## Cleanup

```bash
kubectl delete -f demo-app.yaml
kubectl delete -f external-dns.yaml
kubectl delete -f rbac.yaml
```
