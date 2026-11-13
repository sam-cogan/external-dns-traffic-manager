package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/sam-cogan/external-dns-traffic-manager/pkg/annotations"
	"github.com/sam-cogan/external-dns-traffic-manager/pkg/dnsendpoint"
	"github.com/sam-cogan/external-dns-traffic-manager/pkg/state"
	"github.com/sam-cogan/external-dns-traffic-manager/pkg/trafficmanager"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
)

type trafficManagerClient interface {
	SyncProfilesFromAzure(context.Context, []string) ([]*state.ProfileState, error)
	CreateProfile(context.Context, *trafficmanager.ProfileConfig) (*trafficmanager.ProfileState, error)
	GetProfile(context.Context, string, string) (*trafficmanager.ProfileState, error)
	UpdateProfile(context.Context, *trafficmanager.ProfileConfig) (*trafficmanager.ProfileState, error)
	DeleteProfile(context.Context, string, string) error
	CreateEndpoint(context.Context, string, string, *trafficmanager.EndpointConfig) (*trafficmanager.EndpointState, error)
	UpdateEndpoint(context.Context, string, string, *trafficmanager.EndpointConfig) (*trafficmanager.EndpointState, error)
	DeleteEndpoint(context.Context, string, string, string, string) error
	GetProfileState(context.Context, string, string) (*state.ProfileState, error)
}

type dnsEndpointClient interface {
	CreateOrUpdateCNAME(context.Context, string, string, string, int64) error
	Delete(context.Context, string) error
}

// TrafficManagerProvider implements the webhook provider logic
type TrafficManagerProvider struct {
	domainFilter       []string
	logger             *zap.Logger
	tmClient           trafficManagerClient
	stateManager       *state.Manager
	resourceGroups     []string
	dnsEndpointManager dnsEndpointClient
}

func (p *TrafficManagerProvider) GetDomainFilter() DomainFilter {
	return DomainFilter{Include: append([]string(nil), p.domainFilter...)}
}

// NewTrafficManagerProvider creates a new Traffic Manager provider
func NewTrafficManagerProvider(subscriptionID string, resourceGroups []string, domainFilter []string, k8sClient *kubernetes.Clientset, logger *zap.Logger) (*TrafficManagerProvider, error) {
	// Get Azure credentials
	cred, err := trafficmanager.GetAzureCredential()
	if err != nil {
		return nil, fmt.Errorf("failed to get Azure credentials: %w", err)
	}

	// Test the credential
	ctx := context.Background()
	if err := trafficmanager.TestCredential(ctx, cred); err != nil {
		return nil, fmt.Errorf("failed to validate Azure credentials: %w", err)
	}

	// Create Traffic Manager client
	tmClient, err := trafficmanager.NewClient(subscriptionID, cred, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create Traffic Manager client: %w", err)
	}

	// Create state manager with 5-minute cache TTL
	stateManager := state.NewManager(5*time.Minute, logger)

	// Create DNSEndpoint manager for automatic CNAME creation
	dnsEndpointManager, err := dnsendpoint.NewManager(k8sClient, "default", logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create DNSEndpoint manager: %w", err)
	}

	logger.Info("Successfully initialized Traffic Manager provider",
		zap.String("subscriptionID", subscriptionID),
		zap.Int("resourceGroupCount", len(resourceGroups)))

	return &TrafficManagerProvider{
		domainFilter:       domainFilter,
		logger:             logger,
		tmClient:           tmClient,
		stateManager:       stateManager,
		resourceGroups:     resourceGroups,
		dnsEndpointManager: dnsEndpointManager,
	}, nil
}

// Records returns all Traffic Manager profiles as CNAME records
// This is called by External DNS to get the current state
func (p *TrafficManagerProvider) Records(ctx context.Context) ([]*Endpoint, error) {
	p.logger.Info("Getting records from Traffic Manager")

	// Sync profiles from Azure
	profiles, err := p.tmClient.SyncProfilesFromAzure(ctx, p.resourceGroups)
	if err != nil {
		p.logger.Error("Failed to sync profiles from Azure", zap.Error(err))
		return nil, fmt.Errorf("failed to sync profiles: %w", err)
	}

	// Update state with synced profiles
	for _, profile := range profiles {
		if profile.Hostname != "" {
			p.stateManager.SetProfile(profile.Hostname, profile)
		}
	}

	// Convert profiles to External DNS endpoints
	var endpoints []*Endpoint
	for _, profile := range profiles {
		// Skip profiles without hostname or FQDN
		if profile.Hostname == "" || profile.FQDN == "" {
			p.logger.Debug("Skipping profile without hostname or FQDN",
				zap.String("profileName", profile.ProfileName))
			continue
		}

		// Apply domain filter if configured
		if !p.matchesDomainFilter(profile.Hostname) {
			p.logger.Debug("Profile hostname does not match domain filter",
				zap.String("hostname", profile.Hostname))
			continue
		}

		// Create CNAME endpoint pointing to Traffic Manager FQDN
		endpoint := &Endpoint{
			DNSName:    profile.Hostname,
			Targets:    []string{profile.FQDN},
			RecordType: "CNAME",
			RecordTTL:  300, // 5 minutes
			Labels:     make(map[string]string),
		}

		// Add Traffic Manager metadata as labels
		endpoint.Labels["traffic-manager-profile"] = profile.ProfileName
		endpoint.Labels["traffic-manager-resource-group"] = profile.ResourceGroup
		endpoint.Labels["traffic-manager-routing-method"] = profile.RoutingMethod

		endpoints = append(endpoints, endpoint)
	}

	p.logger.Info("Retrieved Traffic Manager records",
		zap.Int("totalProfiles", len(profiles)),
		zap.Int("endpointCount", len(endpoints)))

	return endpoints, nil
}

// AdjustEndpoints modifies endpoints before they are processed by other providers
// We don't adjust anything - let Azure DNS handle individual service records
// The webhook provider only creates the CNAME for the vanity hostname via Records()
func (p *TrafficManagerProvider) AdjustEndpoints(ctx context.Context, endpoints []*Endpoint) []*Endpoint {
	// Pass through all endpoints unchanged
	// Azure DNS will create A records for individual services (demo-east, demo-west)
	// This webhook creates CNAME for vanity URL (demo) via Records() method
	p.logger.Debug("AdjustEndpoints called - passing through unchanged",
		zap.Int("endpointCount", len(endpoints)))

	return endpoints
}

// ApplyChanges applies the given changes to Traffic Manager
// This is called by External DNS when changes need to be made
func (p *TrafficManagerProvider) ApplyChanges(ctx context.Context, changes *Changes) error {
	if err := validateChanges(changes); err != nil {
		p.logger.Warn("Invalid change batch", zap.Error(err))
		return err
	}
	p.logger.Info("Applying changes to Traffic Manager",
		zap.Int("create", len(changes.Create)),
		zap.Int("updateOld", len(changes.UpdateOld)),
		zap.Int("updateNew", len(changes.UpdateNew)),
		zap.Int("delete", len(changes.Delete)))

	// Process creates
	for _, endpoint := range changes.Create {
		if err := p.createEndpoint(ctx, endpoint); err != nil {
			p.logger.Error("Failed to create endpoint", zap.Error(err))
			return err
		}
	}

	// Process updates
	for i := range changes.UpdateOld {
		if err := p.updateEndpoint(ctx, changes.UpdateOld[i], changes.UpdateNew[i]); err != nil {
			p.logger.Error("Failed to update endpoint", zap.Error(err))
			return err
		}
	}

	// Process deletes
	for _, endpoint := range changes.Delete {
		if err := p.deleteEndpoint(ctx, endpoint); err != nil {
			p.logger.Error("Failed to delete endpoint", zap.Error(err))
			return err
		}
	}

	p.logger.Info("Successfully applied all changes")
	return nil
}

// createEndpoint creates a new Traffic Manager endpoint
func (p *TrafficManagerProvider) createEndpoint(ctx context.Context, endpoint *Endpoint) error {
	p.logger.Info("Creating endpoint",
		zap.String("dnsName", endpoint.DNSName),
		zap.Strings("targets", endpoint.Targets),
		zap.String("recordType", endpoint.RecordType))

	// Skip TXT records - they're for External DNS ownership tracking, not Traffic Manager endpoints
	if endpoint.RecordType == "TXT" {
		p.logger.Debug("Skipping TXT record (ownership record)")
		return nil
	}

	// Debug: Log the full endpoint structure
	p.logger.Debug("Full endpoint details",
		zap.Any("labels", endpoint.Labels),
		zap.Any("providerSpecific", endpoint.ProviderSpecific),
		zap.Int64("ttl", endpoint.RecordTTL))

	// Parse Traffic Manager configuration from annotations
	// Check both Labels and ProviderSpecific (External DNS passes service annotations via ProviderSpecific)
	annotationMap := endpointAnnotations(endpoint)

	p.logger.Debug("Parsing annotations",
		zap.Int("labelCount", len(endpoint.Labels)),
		zap.Int("providerSpecificCount", len(endpoint.ProviderSpecific)),
		zap.Any("annotations", annotationMap))

	config, err := annotations.ParseConfig(annotationMap)
	if err != nil {
		return fmt.Errorf("failed to parse annotations: %w", err)
	}

	// Skip if Traffic Manager is not enabled
	if !config.Enabled {
		p.logger.Debug("Traffic Manager not enabled for this endpoint",
			zap.String("dnsName", endpoint.DNSName))
		return nil
	}

	// Validate configuration
	if err := annotations.ValidateConfig(config); err != nil {
		return fmt.Errorf("invalid Traffic Manager configuration: %w", err)
	}

	// Use vanity hostname if specified, otherwise use endpoint DNSName
	vanityHostname := config.Hostname
	if vanityHostname == "" {
		vanityHostname = endpoint.DNSName
	}

	// Generate profile name if not specified (based on vanity hostname)
	if config.ProfileName == "" {
		config.ProfileName = generateProfileName(vanityHostname)
	}

	// Generate endpoint name if not specified
	if config.EndpointName == "" {
		config.EndpointName = generateEndpointName(endpoint.DNSName, endpoint.Targets)
	}

	p.logger.Info("Creating Traffic Manager profile",
		zap.String("profileName", config.ProfileName),
		zap.String("vanityHostname", vanityHostname),
		zap.String("endpointDNS", endpoint.DNSName),
		zap.String("resourceGroup", config.ResourceGroup))

	// Create or update the Traffic Manager profile
	profileConfig := config.ToProfileConfig()
	// Add hostname tag so we can map Traffic Manager profile back to vanity DNS name
	profileConfig.Tags["hostname"] = vanityHostname
	_, err = p.tmClient.CreateProfile(ctx, profileConfig)
	if err != nil {
		// Profile might already exist, try to get it
		existing, getErr := p.tmClient.GetProfile(ctx, config.ResourceGroup, config.ProfileName)
		if getErr != nil {
			return fmt.Errorf("failed to create/get profile: %w (original error: %v)", getErr, err)
		}
		p.logger.Info("Profile already exists, using existing profile",
			zap.String("profileName", existing.ProfileName),
			zap.String("fqdn", existing.FQDN))
	}

	// Create endpoints for each target
	for i, target := range endpointTargets(endpoint) {
		endpointConfig := config.ToEndpointConfig(target)

		// If we have multiple targets, ensure unique endpoint names
		// This handles the case where External DNS merges multiple DNSEndpoint CRDs
		if len(endpoint.Targets) > 1 && endpointConfig.EndpointName != "" {
			// Append index or target to make it unique
			endpointConfig.EndpointName = fmt.Sprintf("%s-%d", endpointConfig.EndpointName, i)
		} else if endpointConfig.EndpointName == "" {
			// Generate endpoint name from target if not specified
			endpointConfig.EndpointName = generateEndpointNameFromTarget(target, i)
		}

		p.logger.Info("Creating Traffic Manager endpoint",
			zap.String("endpointName", endpointConfig.EndpointName),
			zap.String("target", target),
			zap.Int64("weight", endpointConfig.Weight))

		endpointState, err := p.tmClient.CreateEndpoint(ctx, config.ResourceGroup, config.ProfileName, endpointConfig)
		if err != nil {
			return fmt.Errorf("failed to create endpoint %s: %w", endpointConfig.EndpointName, err)
		}

		// Update state with new endpoint (store under vanity hostname)
		p.stateManager.SetEndpoint(vanityHostname, endpointConfig.EndpointName, convertToStateEndpoint(endpointState))
	}

	// Refresh profile state from Azure to get the complete picture
	profileState, err := p.tmClient.GetProfileState(ctx, config.ResourceGroup, config.ProfileName)
	if err == nil {
		// Store profile under vanity hostname
		profileState.Hostname = vanityHostname
		p.stateManager.SetProfile(vanityHostname, profileState)

		// Automatically create DNSEndpoint CRD for vanity URL CNAME
		if vanityHostname != "" && vanityHostname != endpoint.DNSName && profileState.FQDN != "" {
			dnsEndpointName := dnsendpoint.GenerateName(vanityHostname)
			err = p.dnsEndpointManager.CreateOrUpdateCNAME(ctx, dnsEndpointName, vanityHostname, profileState.FQDN, 300)
			if err != nil {
				p.logger.Error("Failed to create DNSEndpoint for vanity URL",
					zap.String("vanityHostname", vanityHostname),
					zap.String("trafficManagerFQDN", profileState.FQDN),
					zap.Error(err))
				// Don't fail the whole operation if DNSEndpoint creation fails
			} else {
				p.logger.Info("Successfully created DNSEndpoint for vanity URL",
					zap.String("vanityHostname", vanityHostname),
					zap.String("trafficManagerFQDN", profileState.FQDN),
					zap.String("dnsEndpointName", dnsEndpointName))
			}
		}
	}

	p.logger.Info("Successfully created Traffic Manager endpoint",
		zap.String("dnsName", endpoint.DNSName),
		zap.String("vanityHostname", vanityHostname),
		zap.String("profileName", config.ProfileName))

	return nil
}

// updateEndpoint updates an existing Traffic Manager endpoint
func (p *TrafficManagerProvider) updateEndpoint(ctx context.Context, oldEndpoint, newEndpoint *Endpoint) error {
	if newEndpoint.RecordType == "TXT" {
		return nil
	}
	p.logger.Info("Updating endpoint",
		zap.String("dnsName", newEndpoint.DNSName))

	// Parse new configuration
	newConfig, err := annotations.ParseConfig(endpointAnnotations(newEndpoint))
	if err != nil {
		return fmt.Errorf("failed to parse new annotations: %w", err)
	}

	// Skip if Traffic Manager is not enabled
	if !newConfig.Enabled {
		p.logger.Debug("Traffic Manager not enabled for this endpoint",
			zap.String("dnsName", newEndpoint.DNSName))
		return nil
	}

	// Validate configuration
	if err := annotations.ValidateConfig(newConfig); err != nil {
		return fmt.Errorf("invalid Traffic Manager configuration: %w", err)
	}

	// Parse old configuration to detect changes
	oldConfig, err := annotations.ParseConfig(endpointAnnotations(oldEndpoint))
	if err != nil {
		return fmt.Errorf("failed to parse old annotations: %w", err)
	}

	// Use vanity hostname if specified, otherwise use endpoint DNSName
	vanityHostname := newConfig.Hostname
	if vanityHostname == "" {
		vanityHostname = newEndpoint.DNSName
	}

	// Generate names if not specified
	if newConfig.ProfileName == "" {
		newConfig.ProfileName = generateProfileName(vanityHostname)
	}
	if newConfig.EndpointName == "" {
		newConfig.EndpointName = generateEndpointName(newEndpoint.DNSName, newEndpoint.Targets)
	}

	// Check if profile configuration changed
	if oldConfig == nil ||
		oldConfig.RoutingMethod != newConfig.RoutingMethod ||
		oldConfig.DNSTTL != newConfig.DNSTTL ||
		oldConfig.MonitorProtocol != newConfig.MonitorProtocol ||
		oldConfig.MonitorPort != newConfig.MonitorPort ||
		oldConfig.MonitorPath != newConfig.MonitorPath ||
		oldConfig.HealthChecksEnabled != newConfig.HealthChecksEnabled ||
		oldConfig.Hostname != newConfig.Hostname {

		p.logger.Info("Updating Traffic Manager profile",
			zap.String("profileName", newConfig.ProfileName))

		profileConfig := newConfig.ToProfileConfig()
		// Add hostname tag so we can map Traffic Manager profile back to DNS name
		profileConfig.Tags["hostname"] = vanityHostname
		_, err := p.tmClient.UpdateProfile(ctx, profileConfig)
		if err != nil {
			return fmt.Errorf("failed to update profile: %w", err)
		}
	}

	// Update endpoints
	targets := endpointTargets(newEndpoint)
	for i, target := range targets {
		endpointConfig := newConfig.ToEndpointConfig(target)
		if len(targets) > 1 {
			endpointConfig.EndpointName = fmt.Sprintf("%s-%d", endpointConfig.EndpointName, i)
		}

		// Check if we should update weight or status
		if oldConfig != nil &&
			(oldConfig.Weight != newConfig.Weight || oldConfig.EndpointStatus != newConfig.EndpointStatus) {

			p.logger.Info("Updating Traffic Manager endpoint",
				zap.String("endpointName", endpointConfig.EndpointName),
				zap.Int64("weight", endpointConfig.Weight),
				zap.String("status", endpointConfig.Status))

			endpointState, err := p.tmClient.UpdateEndpoint(ctx, newConfig.ResourceGroup, newConfig.ProfileName, endpointConfig)
			if err != nil {
				return fmt.Errorf("failed to update endpoint %s: %w", endpointConfig.EndpointName, err)
			}

			// Update state with modified endpoint
			p.stateManager.SetEndpoint(vanityHostname, endpointConfig.EndpointName, convertToStateEndpoint(endpointState))
		}
	}

	// Refresh complete profile state
	profileState, err := p.tmClient.GetProfileState(ctx, newConfig.ResourceGroup, newConfig.ProfileName)
	if err == nil {
		profileState.Hostname = vanityHostname
		p.stateManager.SetProfile(vanityHostname, profileState)
	}

	p.logger.Info("Successfully updated Traffic Manager endpoint",
		zap.String("dnsName", newEndpoint.DNSName))

	return nil
}

// deleteEndpoint deletes a Traffic Manager endpoint
func (p *TrafficManagerProvider) deleteEndpoint(ctx context.Context, endpoint *Endpoint) error {
	if endpoint.RecordType == "TXT" {
		return nil
	}
	p.logger.Info("Deleting endpoint",
		zap.String("dnsName", endpoint.DNSName))

	// Parse Traffic Manager configuration
	config, err := annotations.ParseConfig(endpointAnnotations(endpoint))
	if err != nil {
		return fmt.Errorf("failed to parse annotations: %w", err)
	}

	// Skip if Traffic Manager is not enabled
	if !config.Enabled {
		p.logger.Debug("Traffic Manager not enabled for this endpoint",
			zap.String("dnsName", endpoint.DNSName))
		return nil
	}

	// Use vanity hostname if specified
	vanityHostname := config.Hostname
	if vanityHostname == "" {
		vanityHostname = endpoint.DNSName
	}

	// Generate names if not specified
	if config.ProfileName == "" {
		config.ProfileName = generateProfileName(vanityHostname)
	}
	if config.EndpointName == "" {
		config.EndpointName = generateEndpointName(endpoint.DNSName, endpoint.Targets)
	}

	// Delete endpoints
	targets := endpointTargets(endpoint)
	for i := range targets {
		endpointName := config.EndpointName
		if len(targets) > 1 {
			endpointName = fmt.Sprintf("%s-%d", endpointName, i)
		}
		p.logger.Info("Deleting Traffic Manager endpoint",
			zap.String("endpointName", endpointName),
			zap.String("profileName", config.ProfileName))

		err := p.tmClient.DeleteEndpoint(ctx, config.ResourceGroup, config.ProfileName, config.EndpointType, endpointName)
		if err != nil {
			p.logger.Warn("Failed to delete endpoint",
				zap.String("endpointName", endpointName),
				zap.Error(err))
			return fmt.Errorf("failed to delete endpoint: %w", err)
		} else {
			// Remove from state
			p.stateManager.DeleteEndpoint(vanityHostname, endpointName)
		}
	}

	// Check if profile still has endpoints
	profileState, err := p.tmClient.GetProfileState(ctx, config.ResourceGroup, config.ProfileName)
	profileMissing := trafficmanager.IsNotFound(err)
	if err != nil && !profileMissing {
		return fmt.Errorf("failed to inspect profile after deletion: %w", err)
	}
	if profileMissing || len(profileState.Endpoints) == 0 {
		// Profile is empty, delete it
		if !profileMissing {
			p.logger.Info("Deleting empty Traffic Manager profile",
				zap.String("profileName", config.ProfileName))

			if err = p.tmClient.DeleteProfile(ctx, config.ResourceGroup, config.ProfileName); err != nil {
				p.logger.Warn("Failed to delete profile",
					zap.String("profileName", config.ProfileName),
					zap.Error(err))
				return fmt.Errorf("failed to delete profile: %w", err)
			}
		}
		p.stateManager.DeleteProfile(vanityHostname)

		// Delete the DNSEndpoint CRD for vanity URL.
		if vanityHostname != "" && vanityHostname != endpoint.DNSName {
			dnsEndpointName := dnsendpoint.GenerateName(vanityHostname)
			if err = p.dnsEndpointManager.Delete(ctx, dnsEndpointName); err != nil {
				p.logger.Warn("Failed to delete DNSEndpoint for vanity URL",
					zap.String("vanityHostname", vanityHostname),
					zap.String("dnsEndpointName", dnsEndpointName),
					zap.Error(err))
				return fmt.Errorf("failed to delete DNSEndpoint: %w", err)
			}
			p.logger.Info("Successfully deleted DNSEndpoint for vanity URL",
				zap.String("vanityHostname", vanityHostname),
				zap.String("dnsEndpointName", dnsEndpointName))
		}
	} else {
		// Profile still has endpoints, update state
		profileState.Hostname = vanityHostname
		p.stateManager.SetProfile(vanityHostname, profileState)
	}

	p.logger.Info("Successfully deleted Traffic Manager endpoint",
		zap.String("dnsName", endpoint.DNSName))

	return nil
}

func endpointAnnotations(endpoint *Endpoint) map[string]string {
	result := make(map[string]string, len(endpoint.Labels)+len(endpoint.ProviderSpecific))
	for key, value := range endpoint.Labels {
		result[key] = value
	}
	for _, property := range endpoint.ProviderSpecific {
		result[property.Name] = property.Value
	}
	return result
}

func endpointTargets(endpoint *Endpoint) []string {
	if endpoint.RecordType == "A" || len(endpoint.Targets) == 0 {
		return []string{endpoint.DNSName}
	}
	return endpoint.Targets
}

func validateChanges(changes *Changes) error {
	if changes == nil {
		return fmt.Errorf("changes must not be null")
	}
	if len(changes.UpdateOld) != len(changes.UpdateNew) {
		return fmt.Errorf("updateOld and updateNew must have the same length")
	}
	for _, endpoints := range [][]*Endpoint{changes.Create, changes.UpdateOld, changes.UpdateNew, changes.Delete} {
		for _, endpoint := range endpoints {
			if endpoint == nil {
				return fmt.Errorf("change endpoints must not be null")
			}
		}
	}
	return nil
}

// generateProfileName generates a profile name from a DNS name
func generateProfileName(dnsName string) string {
	// Remove dots and use as profile name
	// e.g., "myapp.example.com" -> "myapp-example-com"
	return fmt.Sprintf("%s-tm", sanitizeName(dnsName))
}

// generateEndpointName generates an endpoint name from DNS name and target
func generateEndpointName(dnsName string, targets []string) string {
	if len(targets) > 0 {
		return sanitizeName(targets[0])
	}
	return sanitizeName(dnsName)
}

// generateEndpointNameFromTarget generates a unique endpoint name from a target IP/hostname
func generateEndpointNameFromTarget(target string, index int) string {
	// For IPs, replace dots with hyphens
	// For hostnames, sanitize and add index
	sanitized := sanitizeName(target)
	if index > 0 {
		return fmt.Sprintf("%s-%d", sanitized, index)
	}
	return sanitized
}

// sanitizeName sanitizes a string to be used as an Azure resource name
func sanitizeName(name string) string {
	// Replace dots and special characters with hyphens
	sanitized := ""
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			sanitized += string(c)
		} else {
			sanitized += "-"
		}
	}
	return sanitized
}

// convertToStateEndpoint converts trafficmanager.EndpointState to state.EndpointState
func convertToStateEndpoint(tmEndpoint *trafficmanager.EndpointState) *state.EndpointState {
	return &state.EndpointState{
		EndpointName: tmEndpoint.EndpointName,
		EndpointType: tmEndpoint.EndpointType,
		Target:       tmEndpoint.Target,
		Weight:       tmEndpoint.Weight,
		Priority:     tmEndpoint.Priority,
		Status:       tmEndpoint.Status,
		Location:     tmEndpoint.Location,
		CreatedAt:    tmEndpoint.CreatedAt,
		UpdatedAt:    tmEndpoint.UpdatedAt,
	}
}
