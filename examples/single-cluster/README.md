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

1. Update the substitution tokens in `external-dns.yaml`:
   - `__AZURE_SUBSCRIPTION_ID__`, `__AZURE_TENANT_ID__`, and `__AZURE_CLIENT_ID__`
   - `__DNS_ZONE_RESOURCE_GROUP__` and `__TM_RESOURCE_GROUP__`
   - `__CONTAINER_IMAGE__` and `__DOMAIN_FILTER__`

   > [!IMPORTANT]  
   > Make sure to update the `image:` field in the deployment YAML to point to your container registry where you pushed the webhook image.
   > You may also want to update the `--txt-owner-id` field to match your deployment. This field is important and must be unique per cluster, as it is used to determine ownership of the DNS records.

   The legacy `deploy/deploy.sh` script performs these substitutions for you.
   It reads `deploy/azure/infrastructure.env` when present and prompts for any
   missing subscription, DNS/Traffic Manager resource groups, identity IDs,
   registry, and domain values.

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
