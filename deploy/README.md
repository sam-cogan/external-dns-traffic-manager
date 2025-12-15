# Deployment Scripts

This directory contains helper scripts for deploying and testing the External DNS Traffic Manager webhook.

## Directory Structure

- `azure/` - Azure infrastructure setup scripts
- `kubernetes/` - Kubernetes manifest templates (deprecated - use examples/ instead)
- `multi-cluster/` - Multi-cluster deployment scripts
- `deploy.sh` - Main deployment script
- `test.sh` - Test script
- `cleanup.sh` - Cleanup script

## Quick Start

### Single Cluster Deployment

For single cluster deployments, use the examples in the root `examples/single-cluster/` directory instead of these scripts.

```bash
# See examples/single-cluster/README.md for instructions
kubectl apply -f examples/single-cluster/
```

### Multi-Cluster Deployment

For multi-cluster deployments across regions:

```bash
cd multi-cluster

# 1. Configure your settings
vim config.env

# 2. Set up infrastructure (creates AKS clusters, identities, etc.)
./setup-infrastructure.sh

# 3. Deploy webhooks to both clusters
./deploy-webhooks.sh

# 4. Deploy demo apps to both clusters
./deploy-apps.sh

# 5. Verify deployment
./verify.sh
```

See [multi-cluster/README.md](multi-cluster/README.md) for detailed instructions.

## Azure Infrastructure Setup

The `azure/` directory contains scripts for setting up Azure resources:

```bash
cd azure

# Edit configuration
vim infrastructure.env

# Run setup
./setup-infrastructure.sh
```

This creates:
- AKS cluster
- Managed identity with required permissions
- Traffic Manager resource group
- Azure DNS configuration (optional)

## Legacy Deployment (Not Recommended)

The root-level `deploy.sh` script is maintained for backwards compatibility but is deprecated. Use the examples directory instead.

## Cleanup

To remove all deployed resources:

```bash
# Single cluster
./cleanup.sh

# Multi-cluster
cd multi-cluster
./cleanup.sh
```

## Testing

After deployment, run the test script to verify functionality:

```bash
./test.sh
```

This will:
- Check pods are running
- Verify Traffic Manager profiles created
- Test DNS resolution
- Validate endpoint health

## Troubleshooting

Check logs:
```bash
kubectl logs -n external-dns deployment/external-dns -c traffic-manager-webhook
```

Verify Traffic Manager:
```bash
az network traffic-manager profile list --resource-group <tm-rg>
az network traffic-manager endpoint list --profile-name <profile> --resource-group <tm-rg>
```
