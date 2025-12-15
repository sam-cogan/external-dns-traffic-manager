# Troubleshooting Guide

This guide covers common issues and troubleshooting steps for the External DNS Traffic Manager webhook provider.

## Table of Contents

- [Diagnostic Commands](#diagnostic-commands)
- [Common Issues](#common-issues)
- [Debugging Steps](#debugging-steps)
- [Log Analysis](#log-analysis)
- [Azure Verification](#azure-verification)

---

## Diagnostic Commands

### Check Webhook Logs

View the Traffic Manager webhook logs:

```bash
kubectl logs -n external-dns deployment/external-dns -c traffic-manager-webhook
```

For continuous monitoring:
```bash
kubectl logs -n external-dns deployment/external-dns -c traffic-manager-webhook -f
```

### Check External DNS Logs

View the External DNS webhook provider logs:
```bash
kubectl logs -n external-dns deployment/external-dns -c external-dns-webhook
```

View the Azure DNS provider logs:
```bash
kubectl logs -n external-dns deployment/external-dns -c external-dns-azure
```

### Check Pod Status

```bash
# Check if all containers are running
kubectl get pods -n external-dns

# Describe pod for detailed status
kubectl describe pod -n external-dns -l app.kubernetes.io/name=external-dns
```

### Verify Service Configuration

```bash
# Check service annotations
kubectl get svc <service-name> -o yaml | grep -A 20 annotations

# Check service external IP
kubectl get svc <service-name>
```

---

## Common Issues

### Issue: Traffic Manager Profile Not Created

**Symptoms:**
- No Traffic Manager profile appears in Azure portal
- Logs show errors creating profile

**Possible Causes & Solutions:**

1. **Missing Permissions**
   - Check managed identity has Contributor role on Traffic Manager resource group
   ```bash
   az role assignment list \
     --assignee <managed-identity-client-id> \
     --scope /subscriptions/<sub-id>/resourceGroups/<tm-rg>
   ```
   
   - Assign the role if missing:
   ```bash
   az role assignment create \
     --assignee <managed-identity-client-id> \
     --role Contributor \
     --scope /subscriptions/<sub-id>/resourceGroups/<tm-rg>
   ```

2. **Incorrect Annotations**
   - Verify `webhook-traffic-manager-enabled` is set to `"true"` (string, not boolean)
   - Check resource group name is correct and exists
   - Ensure no typos in annotation keys

3. **Authentication Issues**
   - Verify workload identity is properly configured
   - Check AZURE_CLIENT_ID environment variable is set correctly
   - Ensure federated identity credential is created

### Issue: Endpoints Not Being Added

**Symptoms:**
- Profile exists but has no endpoints
- Logs show endpoint creation errors

**Possible Causes & Solutions:**

1. **Service Has No External IP**
   - For LoadBalancer services, wait for external IP to be assigned
   ```bash
   kubectl get svc <service-name> -w
   ```
   
   - Check cloud provider load balancer creation
   ```bash
   kubectl describe svc <service-name>
   ```

2. **Invalid Endpoint Location**
   - Verify the location matches a valid Azure region name
   - Use the short name format (e.g., "eastus", not "East US")
   - List valid locations:
   ```bash
   az account list-locations --query "[].name" -o table
   ```

3. **Missing Required Annotations**
   - Ensure these required annotations are present:
     - `webhook-traffic-manager-enabled: "true"`
     - `webhook-traffic-manager-resource-group`
     - `webhook-traffic-manager-endpoint-location`

### Issue: Health Checks Failing

**Symptoms:**
- Endpoint shows as "Degraded" or "Stopped" in Traffic Manager
- Monitor status shows unhealthy

**Possible Causes & Solutions:**

1. **Monitor Path Returns Non-200 Status**
   - Test the health check endpoint:
   ```bash
   EXTERNAL_IP=$(kubectl get svc <service-name> -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
   curl -v http://$EXTERNAL_IP/<monitor-path>
   ```
   
   - Ensure the path returns HTTP 200 status code

2. **Monitor Port Not Accessible**
   - Verify the service port is correct
   - Check firewall rules allow Traffic Manager health checks
   - Traffic Manager IPs: See [Azure documentation](https://docs.microsoft.com/azure/traffic-manager/traffic-manager-faqs)

3. **Application Not Ready**
   - Check pod readiness:
   ```bash
   kubectl get pods -l app=<your-app>
   ```
   
   - Review application logs for startup errors

4. **SSL/TLS Issues (HTTPS monitors)**
   - Ensure certificate is valid and trusted
   - Check certificate matches the hostname
   - Verify the service supports HTTPS on the specified port

### Issue: DNS Resolution Not Working

**Symptoms:**
- Cannot resolve the hostname
- DNS queries timeout or return NXDOMAIN

**Possible Causes & Solutions:**

1. **CNAME Record Not Created**
   - Check Azure DNS zone for CNAME record:
   ```bash
   az network dns record-set cname show \
     --resource-group <dns-rg> \
     --zone-name <zone> \
     --name <hostname>
   ```
   
   - Check External DNS Azure provider logs for errors

2. **Domain Filter Mismatch**
   - Verify domain-filter in External DNS config includes your domain
   - Check both Azure and webhook providers have matching filters

3. **DNS Propagation Delay**
   - Wait 5-10 minutes for DNS propagation
   - Test with explicit nameserver:
   ```bash
   nslookup <hostname> <azure-dns-server>
   ```

4. **TXT Owner ID Conflicts**
   - Ensure txt-owner-id is unique for each External DNS instance
   - Check for conflicting TXT records in DNS zone

### Issue: Weight or Priority Not Applied

**Symptoms:**
- Traffic distribution doesn't match configured weights
- Priority failover not working as expected

**Possible Causes & Solutions:**

1. **Invalid Weight Value**
   - Weight must be between 1 and 1000
   - Verify annotation value:
   ```bash
   kubectl get svc <service-name> -o jsonpath='{.metadata.annotations.external-dns\.alpha\.kubernetes\.io/webhook-traffic-manager-weight}'
   ```

2. **Routing Method Mismatch**
   - Weight only applies to "Weighted" routing method
   - Priority only applies to "Priority" routing method
   - Check profile routing method:
   ```bash
   az network traffic-manager profile show \
     --name <profile> \
     --resource-group <tm-rg> \
     --query trafficRoutingMethod
   ```

3. **Multiple Endpoints Required**
   - Weighted routing requires at least 2 endpoints
   - Priority routing requires endpoints with different priority values

### Issue: Multiple Clusters Not Sharing Profile

**Symptoms:**
- Each cluster creates its own profile
- Endpoints not appearing in shared profile

**Possible Causes & Solutions:**

1. **Profile Name Mismatch**
   - Verify exact same `webhook-traffic-manager-profile-name` on all services
   - Check for case sensitivity or extra spaces
   - Compare annotations across clusters:
   ```bash
   kubectl --context cluster1 get svc <service> -o yaml | grep profile-name
   kubectl --context cluster2 get svc <service> -o yaml | grep profile-name
   ```

2. **Resource Group Mismatch**
   - Ensure all services specify the same resource group
   - Verify both clusters have access to the resource group

3. **Endpoint Name Collision**
   - Each cluster must use unique endpoint names
   - Recommend naming: `<app>-<region>` or `<app>-<cluster>`

### Issue: Webhook Not Responding

**Symptoms:**
- External DNS logs show connection errors to webhook
- Timeout errors in logs

**Possible Causes & Solutions:**

1. **Container Not Running**
   - Check container status:
   ```bash
   kubectl get pod -n external-dns -o jsonpath='{.items[0].status.containerStatuses[?(@.name=="traffic-manager-webhook")].state}'
   ```

2. **Port Misconfiguration**
   - Verify WEBHOOK_PORT environment variable matches --webhook-provider-url port
   - Default is 8888

3. **Readiness/Liveness Probe Failing**
   - Check probe status:
   ```bash
   kubectl describe pod -n external-dns -l app.kubernetes.io/name=external-dns
   ```

---

## Debugging Steps

### Step-by-Step Debugging Process

1. **Verify Pod is Running**
   ```bash
   kubectl get pods -n external-dns
   ```

2. **Check All Container Logs**
   ```bash
   # Webhook logs
   kubectl logs -n external-dns deployment/external-dns -c traffic-manager-webhook --tail=100
   
   # External DNS webhook provider logs
   kubectl logs -n external-dns deployment/external-dns -c external-dns-webhook --tail=100
   
   # External DNS Azure provider logs
   kubectl logs -n external-dns deployment/external-dns -c external-dns-azure --tail=100
   ```

3. **Verify Service Configuration**
   ```bash
   # Check service has required annotations
   kubectl get svc <service-name> -o yaml
   
   # Check service has external IP
   kubectl get svc <service-name>
   ```

4. **Check Azure Resources**
   ```bash
   # List Traffic Manager profiles
   az network traffic-manager profile list \
     --resource-group <tm-rg> \
     --output table
   
   # Show specific profile
   az network traffic-manager profile show \
     --name <profile> \
     --resource-group <tm-rg>
   
   # List endpoints
   az network traffic-manager endpoint list \
     --profile-name <profile> \
     --resource-group <tm-rg> \
     --type ExternalEndpoints \
     --output table
   ```

5. **Verify DNS Records**
   ```bash
   # Check CNAME record
   az network dns record-set cname show \
     --resource-group <dns-rg> \
     --zone-name <zone> \
     --name <hostname>
   
   # Test DNS resolution
   nslookup <hostname>
   ```

6. **Test Connectivity**
   ```bash
   # Test webhook health endpoint
   kubectl port-forward -n external-dns deployment/external-dns 8888:8888
   curl http://localhost:8888/healthz
   
   # Test application endpoint
   EXTERNAL_IP=$(kubectl get svc <service-name> -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
   curl http://$EXTERNAL_IP/
   ```

---

## Log Analysis

### Understanding Webhook Logs

**Normal Operation:**
```
INFO[0000] Starting webhook server on :8888
INFO[0030] Processing adjustEndpoints request
INFO[0030] Found Traffic Manager config for hostname: app.example.com
INFO[0031] Ensuring Traffic Manager profile: app-profile
INFO[0032] Profile already exists, skipping creation
INFO[0032] Ensuring endpoint: primary in profile app-profile
INFO[0033] Endpoint created successfully
```

**Error Indicators:**

**Authentication Errors:**
```
ERROR[0030] Failed to create Traffic Manager client: DefaultAzureCredential authentication failed
```
→ Check workload identity configuration

**Permission Errors:**
```
ERROR[0031] Failed to create profile: authorization failed
```
→ Check RBAC permissions

**Validation Errors:**
```
WARN[0030] Skipping endpoint: missing required annotation webhook-traffic-manager-resource-group
```
→ Check service annotations

**API Errors:**
```
ERROR[0032] Failed to create endpoint: conflict - endpoint already exists
```
→ May indicate state mismatch or concurrent updates

---

## Azure Verification

### Verify Traffic Manager Profile

```bash
# Show profile details
az network traffic-manager profile show \
  --name <profile-name> \
  --resource-group <tm-rg> \
  --output json

# Key fields to check:
# - trafficRoutingMethod: Should match your annotation
# - monitorConfig: Should match monitor settings
# - dnsConfig.ttl: DNS TTL setting
```

### Verify Endpoint Configuration

```bash
# Show endpoint details
az network traffic-manager endpoint show \
  --name <endpoint-name> \
  --profile-name <profile-name> \
  --resource-group <tm-rg> \
  --type ExternalEndpoints \
  --output json

# Key fields to check:
# - target: Should be service external IP
# - endpointStatus: Should be "Enabled"
# - endpointMonitorStatus: Should be "Online"
# - weight/priority: Should match annotations
# - endpointLocation: Should match configured region
```

### Monitor Endpoint Health

```bash
# Watch endpoint status
watch -n 5 'az network traffic-manager endpoint show \
  --name <endpoint-name> \
  --profile-name <profile-name> \
  --resource-group <tm-rg> \
  --type ExternalEndpoints \
  --query endpointMonitorStatus -o tsv'
```

---

## Getting Help

If you've tried these troubleshooting steps and are still experiencing issues:

1. **Gather Information:**
   - Webhook logs (last 100 lines)
   - Service YAML with annotations
   - Traffic Manager profile and endpoint details
   - External DNS logs

2. **Check Existing Issues:**
   - Review GitHub issues for similar problems
   - Search for error messages in documentation

3. **Create an Issue:**
   - Include all gathered information
   - Describe expected vs actual behavior
   - Include versions of all components

---

## Useful Azure CLI Commands

```bash
# List all Traffic Manager profiles in subscription
az network traffic-manager profile list --output table

# Get all endpoints for a profile
az network traffic-manager endpoint list \
  --profile-name <profile> \
  --resource-group <tm-rg> \
  --output table

# Update endpoint weight
az network traffic-manager endpoint update \
  --name <endpoint> \
  --profile-name <profile> \
  --resource-group <tm-rg> \
  --type ExternalEndpoints \
  --weight <new-weight>

# Disable an endpoint
az network traffic-manager endpoint update \
  --name <endpoint> \
  --profile-name <profile> \
  --resource-group <tm-rg> \
  --type ExternalEndpoints \
  --endpoint-status Disabled

# Delete an endpoint
az network traffic-manager endpoint delete \
  --name <endpoint> \
  --profile-name <profile> \
  --resource-group <tm-rg> \
  --type ExternalEndpoints

# Delete a profile
az network traffic-manager profile delete \
  --name <profile> \
  --resource-group <tm-rg>
```
