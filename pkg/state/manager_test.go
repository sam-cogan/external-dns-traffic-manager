package state

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestNewManager(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cacheTTL := 5 * time.Minute

	manager := NewManager(cacheTTL, logger)

	assert.NotNil(t, manager)
	assert.NotNil(t, manager.profiles)
	assert.Equal(t, cacheTTL, manager.cacheTTL)
	assert.Equal(t, 0, manager.Count())
}

func TestManager_SetAndGetProfile(t *testing.T) {
	logger := zaptest.NewLogger(t)
	manager := NewManager(5*time.Minute, logger)

	profile := &ProfileState{
		ProfileName:   "test-profile",
		ResourceGroup: "test-rg",
		Hostname:      "app.example.com",
		FQDN:          "test-profile.trafficmanager.net",
		RoutingMethod: "Weighted",
		DNSTTL:        30,
		Endpoints:     make(map[string]*EndpointState),
		Tags:          map[string]string{"managedBy": "test"},
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// Set profile
	manager.SetProfile("app.example.com", profile)

	// Get profile
	retrieved, exists := manager.GetProfile("app.example.com")
	require.True(t, exists)
	assert.Equal(t, profile.ProfileName, retrieved.ProfileName)
	assert.Equal(t, profile.ResourceGroup, retrieved.ResourceGroup)
	assert.Equal(t, profile.Hostname, retrieved.Hostname)
	assert.Equal(t, profile.FQDN, retrieved.FQDN)

	// Verify it's a clone (different memory address)
	assert.NotSame(t, profile, retrieved)
}

func TestManager_GetProfile_NotFound(t *testing.T) {
	logger := zaptest.NewLogger(t)
	manager := NewManager(5*time.Minute, logger)

	retrieved, exists := manager.GetProfile("nonexistent.example.com")
	assert.False(t, exists)
	assert.Nil(t, retrieved)
}

func TestManager_GetProfile_Expired(t *testing.T) {
	logger := zaptest.NewLogger(t)
	// Very short TTL for testing
	manager := NewManager(100*time.Millisecond, logger)

	profile := &ProfileState{
		ProfileName: "test-profile",
		Hostname:    "app.example.com",
		CachedAt:    time.Now().Add(-200 * time.Millisecond), // Already expired
	}

	// Manually add to bypass SetProfile which sets CachedAt to now
	manager.profiles["app.example.com"] = profile

	// Should not find expired profile
	retrieved, exists := manager.GetProfile("app.example.com")
	assert.False(t, exists)
	assert.Nil(t, retrieved)
}

func TestManager_DeleteProfile(t *testing.T) {
	logger := zaptest.NewLogger(t)
	manager := NewManager(5*time.Minute, logger)

	profile := &ProfileState{
		ProfileName: "test-profile",
		Hostname:    "app.example.com",
	}

	manager.SetProfile("app.example.com", profile)
	assert.Equal(t, 1, manager.Count())

	manager.DeleteProfile("app.example.com")
	assert.Equal(t, 0, manager.Count())

	retrieved, exists := manager.GetProfile("app.example.com")
	assert.False(t, exists)
	assert.Nil(t, retrieved)
}

func TestManager_ListProfiles(t *testing.T) {
	logger := zaptest.NewLogger(t)
	manager := NewManager(5*time.Minute, logger)

	profiles := []*ProfileState{
		{ProfileName: "profile1", Hostname: "app1.example.com"},
		{ProfileName: "profile2", Hostname: "app2.example.com"},
		{ProfileName: "profile3", Hostname: "app3.example.com"},
	}

	for _, p := range profiles {
		manager.SetProfile(p.Hostname, p)
	}

	retrieved := manager.ListProfiles()
	assert.Len(t, retrieved, 3)

	// Verify all profiles are returned
	hostnames := make(map[string]bool)
	for _, p := range retrieved {
		hostnames[p.Hostname] = true
	}

	assert.True(t, hostnames["app1.example.com"])
	assert.True(t, hostnames["app2.example.com"])
	assert.True(t, hostnames["app3.example.com"])
}

func TestManager_GetProfileByName(t *testing.T) {
	logger := zaptest.NewLogger(t)
	manager := NewManager(5*time.Minute, logger)

	profile := &ProfileState{
		ProfileName: "my-tm-profile",
		Hostname:    "app.example.com",
	}

	manager.SetProfile("app.example.com", profile)

	retrieved, exists := manager.GetProfileByName("my-tm-profile")
	require.True(t, exists)
	assert.Equal(t, "my-tm-profile", retrieved.ProfileName)
	assert.Equal(t, "app.example.com", retrieved.Hostname)
}

func TestManager_GetProfileByName_NotFound(t *testing.T) {
	logger := zaptest.NewLogger(t)
	manager := NewManager(5*time.Minute, logger)

	retrieved, exists := manager.GetProfileByName("nonexistent")
	assert.False(t, exists)
	assert.Nil(t, retrieved)
}

func TestManager_SetAndGetEndpoint(t *testing.T) {
	logger := zaptest.NewLogger(t)
	manager := NewManager(5*time.Minute, logger)

	profile := &ProfileState{
		ProfileName: "test-profile",
		Hostname:    "app.example.com",
		Endpoints:   make(map[string]*EndpointState),
	}
	manager.SetProfile("app.example.com", profile)

	endpoint := &EndpointState{
		EndpointName: "endpoint1",
		Target:       "20.30.40.50",
		Weight:       100,
		Status:       "Enabled",
	}

	manager.SetEndpoint("app.example.com", "endpoint1", endpoint)

	retrieved, exists := manager.GetEndpoint("app.example.com", "endpoint1")
	require.True(t, exists)
	assert.Equal(t, "endpoint1", retrieved.EndpointName)
	assert.Equal(t, "20.30.40.50", retrieved.Target)
	assert.Equal(t, int64(100), retrieved.Weight)
}

func TestManager_DeleteEndpoint(t *testing.T) {
	logger := zaptest.NewLogger(t)
	manager := NewManager(5*time.Minute, logger)

	profile := &ProfileState{
		ProfileName: "test-profile",
		Hostname:    "app.example.com",
		Endpoints: map[string]*EndpointState{
			"endpoint1": {EndpointName: "endpoint1"},
		},
	}
	manager.SetProfile("app.example.com", profile)

	manager.DeleteEndpoint("app.example.com", "endpoint1")

	retrieved, exists := manager.GetEndpoint("app.example.com", "endpoint1")
	assert.False(t, exists)
	assert.Nil(t, retrieved)
}

func TestManager_Clear(t *testing.T) {
	logger := zaptest.NewLogger(t)
	manager := NewManager(5*time.Minute, logger)

	for i := 1; i <= 5; i++ {
		profile := &ProfileState{
			ProfileName: "profile" + string(rune('0'+i)),
			Hostname:    "app" + string(rune('0'+i)) + ".example.com",
		}
		manager.SetProfile(profile.Hostname, profile)
	}

	assert.Equal(t, 5, manager.Count())

	manager.Clear()

	assert.Equal(t, 0, manager.Count())
	assert.Empty(t, manager.ListProfiles())
}

func TestManager_GetStats(t *testing.T) {
	logger := zaptest.NewLogger(t)
	manager := NewManager(5*time.Minute, logger)

	// Add profiles with endpoints
	profile1 := &ProfileState{
		ProfileName: "profile1",
		Hostname:    "app1.example.com",
		Endpoints: map[string]*EndpointState{
			"ep1": {EndpointName: "ep1"},
			"ep2": {EndpointName: "ep2"},
		},
	}
	profile2 := &ProfileState{
		ProfileName: "profile2",
		Hostname:    "app2.example.com",
		Endpoints: map[string]*EndpointState{
			"ep3": {EndpointName: "ep3"},
		},
	}

	manager.SetProfile("app1.example.com", profile1)
	manager.SetProfile("app2.example.com", profile2)

	stats := manager.GetStats()

	assert.Equal(t, 2, stats["totalProfiles"])
	assert.Equal(t, 3, stats["totalEndpoints"])
	assert.Equal(t, 0, stats["expiredProfiles"])
	assert.NotEmpty(t, stats["cacheTTL"])
}

func TestManager_ConcurrentAccess(t *testing.T) {
	logger := zaptest.NewLogger(t)
	manager := NewManager(5*time.Minute, logger)

	// Test concurrent writes and reads
	done := make(chan bool)
	iterations := 100

	// Concurrent writers
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < iterations; j++ {
				profile := &ProfileState{
					ProfileName: "profile",
					Hostname:    "app.example.com",
				}
				manager.SetProfile("app.example.com", profile)
			}
			done <- true
		}(i)
	}

	// Concurrent readers
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < iterations; j++ {
				manager.GetProfile("app.example.com")
				manager.ListProfiles()
				manager.Count()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}

	// Should not panic and should have the profile
	profile, exists := manager.GetProfile("app.example.com")
	assert.True(t, exists)
	assert.NotNil(t, profile)
}

func TestProfileState_Clone(t *testing.T) {
	original := &ProfileState{
		ProfileName:   "test-profile",
		ResourceGroup: "test-rg",
		Hostname:      "app.example.com",
		FQDN:          "test.trafficmanager.net",
		Endpoints: map[string]*EndpointState{
			"ep1": {EndpointName: "ep1", Target: "1.2.3.4"},
		},
		Tags: map[string]string{"key": "value"},
	}

	cloned := original.Clone()

	// Verify values are equal
	assert.Equal(t, original.ProfileName, cloned.ProfileName)
	assert.Equal(t, original.Hostname, cloned.Hostname)

	// Verify it's a deep clone
	assert.NotSame(t, original, cloned)
	assert.NotSame(t, original.Endpoints, cloned.Endpoints)
	assert.NotSame(t, original.Tags, cloned.Tags)

	// Modify clone shouldn't affect original
	cloned.ProfileName = "modified"
	cloned.Tags["newkey"] = "newvalue"
	delete(cloned.Endpoints, "ep1")

	assert.Equal(t, "test-profile", original.ProfileName)
	assert.Equal(t, 1, len(original.Endpoints))
	assert.NotContains(t, original.Tags, "newkey")
}

func TestEndpointState_Clone(t *testing.T) {
	original := &EndpointState{
		EndpointName: "test-endpoint",
		Target:       "1.2.3.4",
		Weight:       100,
		Status:       "Enabled",
	}

	cloned := original.Clone()

	// Verify values are equal
	assert.Equal(t, original.EndpointName, cloned.EndpointName)
	assert.Equal(t, original.Target, cloned.Target)
	assert.Equal(t, original.Weight, cloned.Weight)

	// Verify it's a different instance
	assert.NotSame(t, original, cloned)

	// Modify clone shouldn't affect original
	cloned.EndpointName = "modified"
	cloned.Weight = 200

	assert.Equal(t, "test-endpoint", original.EndpointName)
	assert.Equal(t, int64(100), original.Weight)
}

func TestProfileState_IsExpired(t *testing.T) {
	ttl := 5 * time.Minute

	tests := []struct {
		name     string
		cachedAt time.Time
		ttl      time.Duration
		expected bool
	}{
		{
			name:     "Fresh cache",
			cachedAt: time.Now(),
			ttl:      ttl,
			expected: false,
		},
		{
			name:     "Just expired",
			cachedAt: time.Now().Add(-ttl - time.Second),
			ttl:      ttl,
			expected: true,
		},
		{
			name:     "Not cached yet (zero time)",
			cachedAt: time.Time{},
			ttl:      ttl,
			expected: true,
		},
		{
			name:     "Way expired",
			cachedAt: time.Now().Add(-24 * time.Hour),
			ttl:      ttl,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := &ProfileState{
				CachedAt: tt.cachedAt,
			}
			assert.Equal(t, tt.expected, profile.IsExpired(tt.ttl))
		})
	}
}
