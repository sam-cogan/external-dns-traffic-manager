package annotations

const (
	// AnnotationPrefix is the common prefix for all Traffic Manager annotations
	// ExternalDNS transforms annotation "webhook-" suffixes to "webhook/"
	// provider-specific properties; this is the wire prefix, not an annotation key.
	AnnotationPrefix = "webhook/traffic-manager-"

	// Core configuration annotations
	AnnotationEnabled       = AnnotationPrefix + "enabled"
	AnnotationProfileName   = AnnotationPrefix + "profile-name"
	AnnotationResourceGroup = AnnotationPrefix + "resource-group"
	AnnotationHostname      = AnnotationPrefix + "hostname"

	// Routing configuration
	AnnotationRoutingMethod = AnnotationPrefix + "routing-method"
	AnnotationWeight        = AnnotationPrefix + "weight"
	AnnotationPriority      = AnnotationPrefix + "priority"

	// Endpoint configuration
	AnnotationEndpointName     = AnnotationPrefix + "endpoint-name"
	AnnotationEndpointLocation = AnnotationPrefix + "endpoint-location"
	AnnotationEndpointStatus   = AnnotationPrefix + "endpoint-status"

	// DNS configuration
	AnnotationDNSTTL = AnnotationPrefix + "dns-ttl"

	// Monitoring configuration
	AnnotationMonitorProtocol     = AnnotationPrefix + "monitor-protocol"
	AnnotationMonitorPort         = AnnotationPrefix + "monitor-port"
	AnnotationMonitorPath         = AnnotationPrefix + "monitor-path"
	AnnotationHealthChecksEnabled = AnnotationPrefix + "health-checks-enabled"
)

// Default values
const (
	DefaultRoutingMethod       = "Weighted"
	DefaultWeight              = int64(100)
	DefaultPriority            = int64(1)
	DefaultDNSTTL              = int64(30)
	DefaultMonitorProtocol     = "HTTPS"
	DefaultMonitorPort         = int64(443)
	DefaultMonitorPath         = "/"
	DefaultEndpointStatus      = "Enabled"
	DefaultEndpointType        = "ExternalEndpoints"
	DefaultHealthChecksEnabled = true
)
