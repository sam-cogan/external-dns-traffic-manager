package state

import (
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestEndpointStateEdgeCases(t *testing.T) {
	manager := NewManager(time.Minute, zap.NewNop())
	manager.SetEndpoint("missing", "east", &EndpointState{})
	manager.DeleteEndpoint("missing", "east")
	manager.SetProfile("app.example.com", &ProfileState{CachedAt: time.Now()})
	manager.SetEndpoint("app.example.com", "east", &EndpointState{EndpointName: "east"})
	if _, ok := manager.GetEndpoint("app.example.com", "missing"); ok {
		t.Fatal("nonexistent endpoint returned")
	}
	manager.DeleteEndpoint("app.example.com", "east")
	if _, ok := manager.GetEndpoint("app.example.com", "east"); ok {
		t.Fatal("deleted endpoint returned")
	}
}
