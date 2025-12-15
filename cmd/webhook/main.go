package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sam-cogan/external-dns-traffic-manager/pkg/provider"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	// Initialize logger
	logger, err := initLogger()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("Starting Traffic Manager Webhook Provider")

	// Get configuration from environment
	config := getConfig()
	logger.Info("Configuration loaded",
		zap.String("webhookPort", config.WebhookPort),
		zap.String("healthPort", config.HealthPort),
		zap.Strings("domainFilter", config.DomainFilter))

	// Validate required configuration
	if config.SubscriptionID == "" {
		logger.Fatal("AZURE_SUBSCRIPTION_ID environment variable is required")
	}

	if len(config.ResourceGroups) == 0 {
		logger.Warn("RESOURCE_GROUPS not configured - will not sync existing profiles from Azure")
	}

	// Create Kubernetes client
	k8sClient, err := createKubernetesClient()
	if err != nil {
		logger.Fatal("Failed to create Kubernetes client", zap.Error(err))
	}

	// Create Traffic Manager provider
	tmProvider, err := provider.NewTrafficManagerProvider(config.SubscriptionID, config.ResourceGroups, config.DomainFilter, k8sClient, logger)
	if err != nil {
		logger.Fatal("Failed to create Traffic Manager provider", zap.Error(err))
	}

	// Create webhook server
	webhookServer := provider.NewWebhookServer(tmProvider, logger)

	// Set up HTTP routes for webhook endpoints (localhost only)
	webhookMux := http.NewServeMux()
	webhookMux.HandleFunc("/", webhookServer.HandleNegotiate)
	webhookMux.HandleFunc("/records", webhookServer.HandleRecords)
	webhookMux.HandleFunc("/adjustendpoints", webhookServer.HandleAdjustEndpoints)

	// Set up HTTP routes for health/metrics endpoints (all interfaces)
	healthMux := http.NewServeMux()
	healthMux.HandleFunc("/healthz", webhookServer.HandleHealth)
	healthMux.HandleFunc("/readyz", webhookServer.HandleHealth) // Readiness probe uses same health check
	healthMux.HandleFunc("/metrics", handleMetrics)

	// Create HTTP servers
	webhookHTTPServer := &http.Server{
		Addr:         fmt.Sprintf("0.0.0.0:%s", config.WebhookPort),
		Handler:      webhookMux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	healthHTTPServer := &http.Server{
		Addr:         fmt.Sprintf("0.0.0.0:%s", config.HealthPort),
		Handler:      healthMux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Channel to listen for errors from servers
	serverErrors := make(chan error, 2)

	// Start webhook server
	go func() {
		logger.Info("Starting webhook server", zap.String("address", webhookHTTPServer.Addr))
		serverErrors <- webhookHTTPServer.ListenAndServe()
	}()

	// Start health server
	go func() {
		logger.Info("Starting health server", zap.String("address", healthHTTPServer.Addr))
		serverErrors <- healthHTTPServer.ListenAndServe()
	}()

	// Set up graceful shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// Block until we receive a signal or error
	select {
	case err := <-serverErrors:
		if err != http.ErrServerClosed {
			logger.Fatal("Server error", zap.Error(err))
		}
	case sig := <-shutdown:
		logger.Info("Received shutdown signal", zap.String("signal", sig.String()))
	}

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	logger.Info("Shutting down servers...")

	if err := webhookHTTPServer.Shutdown(ctx); err != nil {
		logger.Error("Webhook server shutdown error", zap.Error(err))
	}

	if err := healthHTTPServer.Shutdown(ctx); err != nil {
		logger.Error("Health server shutdown error", zap.Error(err))
	}

	logger.Info("Servers stopped")
}

// Config holds the application configuration
type Config struct {
	WebhookPort      string
	HealthPort       string
	DomainFilter     []string
	ResourceGroups   []string
	SubscriptionID   string
	TenantID         string
	ClientID         string
	ClientSecret     string
	LogLevel         string
}

// getConfig loads configuration from environment variables
func getConfig() *Config {
	return &Config{
		WebhookPort:      getEnv("WEBHOOK_PORT", "8888"),
		HealthPort:       getEnv("HEALTH_PORT", "8080"),
		DomainFilter:     getEnvSlice("DOMAIN_FILTER", []string{}),
		ResourceGroups:   getEnvSlice("RESOURCE_GROUPS", []string{}),
		SubscriptionID:   getEnv("AZURE_SUBSCRIPTION_ID", ""),
		TenantID:         getEnv("AZURE_TENANT_ID", ""),
		ClientID:         getEnv("AZURE_CLIENT_ID", ""),
		ClientSecret:     getEnv("AZURE_CLIENT_SECRET", ""),
		LogLevel:         getEnv("LOG_LEVEL", "info"),
	}
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvSlice gets an environment variable as a slice (comma-separated)
func getEnvSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		// Simple split by comma - could be enhanced
		result := []string{}
		current := ""
		for _, char := range value {
			if char == ',' {
				if current != "" {
					result = append(result, current)
					current = ""
				}
			} else {
				current += string(char)
			}
		}
		if current != "" {
			result = append(result, current)
		}
		return result
	}
	return defaultValue
}

// initLogger initializes the logger based on environment
func initLogger() (*zap.Logger, error) {
	logLevel := getEnv("LOG_LEVEL", "info")

	var config zap.Config
	if os.Getenv("ENVIRONMENT") == "production" {
		config = zap.NewProductionConfig()
	} else {
		config = zap.NewDevelopmentConfig()
	}

	// Set log level
	switch logLevel {
	case "debug":
		config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "info":
		config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	case "warn":
		config.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		config.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	default:
		config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	return config.Build()
}

// createKubernetesClient creates a Kubernetes client for the in-cluster environment
func createKubernetesClient() (*kubernetes.Clientset, error) {
	// Try in-cluster config first
	config, err := rest.InClusterConfig()
	if err != nil {
		// Fall back to kubeconfig for local development
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			kubeconfig = os.Getenv("HOME") + "/.kube/config"
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create kubernetes config: %w", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	return clientset, nil
}

// handleMetrics is a placeholder for metrics endpoint
func handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// TODO: Implement Prometheus metrics
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "# Metrics endpoint - TODO: Implement Prometheus metrics\n")
}
