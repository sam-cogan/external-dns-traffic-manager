package dnsendpoint

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	ktesting "k8s.io/client-go/testing"
)

func TestDeleteTreatsNotFoundAsSuccess(t *testing.T) {
	client := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	resource := DNSEndpointGVR()
	client.PrependReactor("delete", resource.Resource, func(ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewNotFound(
			schema.GroupResource{Group: resource.Group, Resource: resource.Resource},
			"app-example-com-tm-cname",
		)
	})
	manager := &Manager{client: client, namespace: "external-dns", logger: zap.NewNop()}

	if err := manager.Delete(context.Background(), "app-example-com-tm-cname"); err != nil {
		t.Fatalf("deleting an already-absent DNSEndpoint: %v", err)
	}
}

func TestDeletePropagatesOtherErrors(t *testing.T) {
	client := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	wantErr := errors.New("forbidden")
	client.PrependReactor("delete", DNSEndpointGVR().Resource, func(ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, wantErr
	})
	manager := &Manager{client: client, namespace: "external-dns", logger: zap.NewNop()}

	if err := manager.Delete(context.Background(), "app-example-com-tm-cname"); !errors.Is(err, wantErr) {
		t.Fatalf("delete error = %v, want wrapped %v", err, wantErr)
	}
}
