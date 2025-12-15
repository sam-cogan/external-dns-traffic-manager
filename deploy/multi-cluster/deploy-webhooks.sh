#!/bin/bash
set -e

# Load configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/config.env"

echo "=========================================="
echo "Deploying External DNS Webhook to Clusters"
echo "=========================================="
echo ""

# Function to deploy to a cluster
deploy_to_cluster() {
    local CLUSTER_NAME=$1
    local CLUSTER_CONTEXT=$2
    local REGION=$3
    
    echo "Deploying to ${CLUSTER_NAME} (${REGION})..."
    kubectl config use-context "${CLUSTER_CONTEXT}"
    
    # Create namespace
    kubectl create namespace external-dns --dry-run=client -o yaml | kubectl apply -f -
    
    # Apply CRD
    kubectl apply -f "${SCRIPT_DIR}/../kubernetes/dnsendpoint-crd.yaml"
    
    # Apply RBAC
    kubectl apply -f "${SCRIPT_DIR}/../kubernetes/rbac.yaml"
    
    # Create Azure credentials secret
    echo "Creating Azure credentials..."
    AZURE_SUBSCRIPTION_ID="${SUBSCRIPTION_ID}"
    AZURE_TENANT_ID=$(az account show --query tenantId -o tsv)
    AZURE_RESOURCE_GROUP="${TM_PROFILE_RG}"
    
    kubectl create secret generic azure-config \
        --from-literal=azure.json="{
            \"tenantId\": \"${AZURE_TENANT_ID}\",
            \"subscriptionId\": \"${AZURE_SUBSCRIPTION_ID}\",
            \"resourceGroup\": \"${DNS_ZONE_RG}\",
            \"useManagedIdentityExtension\": true
        }" \
        --namespace external-dns \
        --dry-run=client -o yaml | kubectl apply -f -
    
    # Create webhook config
    kubectl create configmap webhook-config \
        --from-literal=AZURE_SUBSCRIPTION_ID="${AZURE_SUBSCRIPTION_ID}" \
        --from-literal=AZURE_RESOURCE_GROUPS="${TM_PROFILE_RG}" \
        --from-literal=DOMAIN_FILTER="${DNS_ZONE_NAME}" \
        --namespace external-dns \
        --dry-run=client -o yaml | kubectl apply -f -
    
    # Delete existing deployment if selector needs to change
    kubectl delete deployment external-dns -n external-dns --ignore-not-found=true
    
    # Wait for deletion to complete
    kubectl wait --for=delete deployment/external-dns -n external-dns --timeout=60s 2>/dev/null || true
    
    # Deploy webhook deployment
    cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: external-dns
  namespace: external-dns
  labels:
    app.kubernetes.io/name: external-dns
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: external-dns
  template:
    metadata:
      labels:
        app.kubernetes.io/name: external-dns
    spec:
      serviceAccountName: external-dns
      containers:
      # Azure DNS Provider - manages A records for individual endpoints
      - name: external-dns-azure
        image: registry.k8s.io/external-dns/external-dns:v0.20.0
        args:
        - --source=service
        - --source=crd
        - --domain-filter=${DNS_ZONE_NAME}
        - --provider=azure
        - --azure-resource-group=${DNS_ZONE_RG}
        - --azure-subscription-id=${AZURE_SUBSCRIPTION_ID}
        - --txt-owner-id=external-dns-tm
        - --txt-prefix=a-
        - --registry=txt
        - --interval=1m
        - --log-level=debug
        - --metrics-address=:7979
        volumeMounts:
        - name: azure-config
          mountPath: /etc/kubernetes
          readOnly: true
        resources:
          requests:
            cpu: 50m
            memory: 64Mi
          limits:
            cpu: 100m
            memory: 128Mi
      
      # External DNS Webhook - calls Traffic Manager webhook
      - name: external-dns-webhook
        image: registry.k8s.io/external-dns/external-dns:v0.20.0
        args:
        - --source=service
        - --source=crd
        - --domain-filter=${DNS_ZONE_NAME}
        - --provider=webhook
        - --webhook-provider-url=http://localhost:8888
        - --registry=noop
        - --txt-owner-id=external-dns-tm-webhook
        - --interval=1m
        - --log-level=debug
        - --metrics-address=:7980
        resources:
          requests:
            cpu: 50m
            memory: 64Mi
          limits:
            cpu: 100m
            memory: 128Mi
      
      # Traffic Manager Webhook Provider
      - name: traffic-manager-webhook
        image: ${IMAGE_REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG}
        imagePullPolicy: Always
        ports:
        - containerPort: 8888
          name: webhook
        - containerPort: 8080
          name: health
        env:
        - name: WEBHOOK_HOST
          value: "0.0.0.0"
        - name: WEBHOOK_PORT
          value: "8888"
        - name: HEALTH_PORT
          value: "8080"
        - name: DOMAIN_FILTER
          value: "${DNS_ZONE_NAME}"
        - name: AZURE_SUBSCRIPTION_ID
          value: "${AZURE_SUBSCRIPTION_ID}"
        - name: AZURE_RESOURCE_GROUPS
          value: "${TM_PROFILE_RG}"
        - name: AZURE_TENANT_ID
          value: "${AZURE_TENANT_ID}"
        - name: AZURE_USE_MANAGED_IDENTITY
          value: "true"
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 200m
            memory: 256Mi
      
      volumes:
      - name: azure-config
        secret:
          secretName: azure-config
EOF
    
    # Wait for deployment
    echo "Waiting for External DNS to be ready..."
    kubectl wait --for=condition=available --timeout=120s deployment/external-dns -n external-dns
    
    echo "✓ Deployed to ${CLUSTER_NAME}"
    echo ""
}

# Deploy to both clusters
deploy_to_cluster "${CLUSTER_EAST_NAME}" "aks-tm-east" "${CLUSTER_EAST_REGION}"
deploy_to_cluster "${CLUSTER_WEST_NAME}" "aks-tm-west" "${CLUSTER_WEST_REGION}"

echo "=========================================="
echo "✓ Webhook Deployment Complete"
echo "=========================================="
echo ""
echo "Verify deployments:"
echo "  kubectl --context aks-tm-east get pods -n external-dns"
echo "  kubectl --context aks-tm-west get pods -n external-dns"
echo ""
echo "Next step:"
echo "  ./deploy-apps.sh"
