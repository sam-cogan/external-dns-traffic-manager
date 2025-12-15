package state

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager manages the state of Traffic Manager profiles
type Manager struct {
	profiles map[string]*ProfileState // Map of hostname to profile state
	mu       sync.RWMutex
	logger   *zap.Logger
	cacheTTL time.Duration
}

// NewManager creates a new state manager
func NewManager(cacheTTL time.Duration, logger *zap.Logger) *Manager {
	return &Manager{
		profiles: make(map[string]*ProfileState),
		logger:   logger,
		cacheTTL: cacheTTL,
	}
}

// GetProfile retrieves a profile by hostname
func (m *Manager) GetProfile(hostname string) (*ProfileState, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	profile, exists := m.profiles[hostname]
	if !exists {
		return nil, false
	}

	// Check if cache is expired
	if profile.IsExpired(m.cacheTTL) {
		m.logger.Debug("Profile cache expired",
			zap.String("hostname", hostname),
			zap.Time("cachedAt", profile.CachedAt))
		return nil, false
	}

	return profile.Clone(), true
}

// SetProfile stores or updates a profile
func (m *Manager) SetProfile(hostname string, profile *ProfileState) {
	m.mu.Lock()
	defer m.mu.Unlock()

	profile.CachedAt = time.Now()
	m.profiles[hostname] = profile.Clone()

	m.logger.Debug("Profile state updated",
		zap.String("hostname", hostname),
		zap.String("profileName", profile.ProfileName),
		zap.Int("endpointCount", len(profile.Endpoints)))
}

// DeleteProfile removes a profile from state
func (m *Manager) DeleteProfile(hostname string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.profiles, hostname)

	m.logger.Debug("Profile state deleted",
		zap.String("hostname", hostname))
}

// ListProfiles returns all profiles
func (m *Manager) ListProfiles() []*ProfileState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	profiles := make([]*ProfileState, 0, len(m.profiles))
	for _, profile := range m.profiles {
		profiles = append(profiles, profile.Clone())
	}

	return profiles
}

// GetProfileByName retrieves a profile by its Traffic Manager profile name
func (m *Manager) GetProfileByName(profileName string) (*ProfileState, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, profile := range m.profiles {
		if profile.ProfileName == profileName {
			return profile.Clone(), true
		}
	}

	return nil, false
}

// Clear removes all profiles from state
func (m *Manager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.profiles = make(map[string]*ProfileState)

	m.logger.Debug("State cleared")
}

// Count returns the number of profiles in state
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.profiles)
}

// GetEndpoint retrieves an endpoint from a profile
func (m *Manager) GetEndpoint(hostname, endpointName string) (*EndpointState, bool) {
	profile, exists := m.GetProfile(hostname)
	if !exists {
		return nil, false
	}

	endpoint, exists := profile.Endpoints[endpointName]
	if !exists {
		return nil, false
	}

	return endpoint.Clone(), true
}

// SetEndpoint updates or adds an endpoint to a profile
func (m *Manager) SetEndpoint(hostname, endpointName string, endpoint *EndpointState) {
	m.mu.Lock()
	defer m.mu.Unlock()

	profile, exists := m.profiles[hostname]
	if !exists {
		m.logger.Warn("Attempted to set endpoint for non-existent profile",
			zap.String("hostname", hostname),
			zap.String("endpointName", endpointName))
		return
	}

	if profile.Endpoints == nil {
		profile.Endpoints = make(map[string]*EndpointState)
	}

	profile.Endpoints[endpointName] = endpoint.Clone()
	profile.UpdatedAt = time.Now()
	profile.CachedAt = time.Now()

	m.logger.Debug("Endpoint state updated",
		zap.String("hostname", hostname),
		zap.String("endpointName", endpointName))
}

// DeleteEndpoint removes an endpoint from a profile
func (m *Manager) DeleteEndpoint(hostname, endpointName string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	profile, exists := m.profiles[hostname]
	if !exists {
		return
	}

	delete(profile.Endpoints, endpointName)
	profile.UpdatedAt = time.Now()
	profile.CachedAt = time.Now()

	m.logger.Debug("Endpoint state deleted",
		zap.String("hostname", hostname),
		zap.String("endpointName", endpointName))
}

// GetStats returns statistics about the current state
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalEndpoints := 0
	expiredProfiles := 0

	for _, profile := range m.profiles {
		totalEndpoints += len(profile.Endpoints)
		if profile.IsExpired(m.cacheTTL) {
			expiredProfiles++
		}
	}

	return map[string]interface{}{
		"totalProfiles":    len(m.profiles),
		"totalEndpoints":   totalEndpoints,
		"expiredProfiles":  expiredProfiles,
		"cacheTTL":         m.cacheTTL.String(),
	}
}
