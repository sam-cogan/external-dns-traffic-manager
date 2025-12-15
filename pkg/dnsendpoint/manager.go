package dnsendpoint

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Manager handles DNSEndpoint CRD operations
type Manager struct {
	client    dynamic.Interface
	namespace string
	logger    *zap.Logger
}

// NewManager creates a new DNSEndpoint manager
func NewManager(k8sClient *kubernetes.Clientset, namespace string, logger *zap.Logger) (*Manager, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	return &Manager{
		client:    dynamicClient,
		namespace: namespace,
		logger:    logger,
	}, nil
}

// DNSEndpointGVR returns the GroupVersionResource for DNSEndpoint
func DNSEndpointGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "externaldns.k8s.io",
		Version:  "v1alpha1",
		Resource: "dnsendpoints",
	}
}

// CreateOrUpdateCNAME creates or updates a DNSEndpoint for a CNAME record
func (m *Manager) CreateOrUpdateCNAME(ctx context.Context, name, hostname, target string, ttl int64) error {
	m.logger.Info("Creating or updating DNSEndpoint for CNAME",
		zap.String("name", name),
		zap.String("hostname", hostname),
		zap.String("target", target))

	// Create the DNSEndpoint object
	dnsEndpoint := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "externaldns.k8s.io/v1alpha1",
			"kind":       "DNSEndpoint",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": m.namespace,
				"labels": map[string]interface{}{
					"app.kubernetes.io/managed-by": "external-dns-traffic-manager-webhook",
				},
			},
			"spec": map[string]interface{}{
				"endpoints": []interface{}{
					map[string]interface{}{
						"dnsName":    hostname,
						"recordTTL":  ttl,
						"recordType": "CNAME",
						"targets": []interface{}{
							target,
						},
					},
				},
			},
		},
	}

	// Try to get existing DNSEndpoint
	existing, err := m.client.Resource(DNSEndpointGVR()).Namespace(m.namespace).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		// Update existing
		m.logger.Debug("Updating existing DNSEndpoint", zap.String("name", name))
		dnsEndpoint.SetResourceVersion(existing.GetResourceVersion())
		_, err = m.client.Resource(DNSEndpointGVR()).Namespace(m.namespace).Update(ctx, dnsEndpoint, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update DNSEndpoint: %w", err)
		}
		m.logger.Info("Successfully updated DNSEndpoint", zap.String("name", name))
	} else {
		// Create new
		m.logger.Debug("Creating new DNSEndpoint", zap.String("name", name))
		_, err = m.client.Resource(DNSEndpointGVR()).Namespace(m.namespace).Create(ctx, dnsEndpoint, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create DNSEndpoint: %w", err)
		}
		m.logger.Info("Successfully created DNSEndpoint", zap.String("name", name))
	}

	return nil
}

// Delete removes a DNSEndpoint
func (m *Manager) Delete(ctx context.Context, name string) error {
	m.logger.Info("Deleting DNSEndpoint", zap.String("name", name))

	err := m.client.Resource(DNSEndpointGVR()).Namespace(m.namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete DNSEndpoint: %w", err)
	}

	m.logger.Info("Successfully deleted DNSEndpoint", zap.String("name", name))
	return nil
}

// GenerateName generates a DNSEndpoint name from a hostname
func GenerateName(hostname string) string {
	// Replace dots with hyphens and add suffix
	name := ""
	for _, c := range hostname {
		if c == '.' {
			name += "-"
		} else if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' {
			name += string(c)
		}
	}
	return name + "-tm-cname"
}
