package provider

import (
	"encoding/json"
	"fmt"
	"net/http"

	"go.uber.org/zap"
)

// WebhookServer handles HTTP requests for the webhook provider
type WebhookServer struct {
	provider *TrafficManagerProvider
	logger   *zap.Logger
}

// NewWebhookServer creates a new webhook server
func NewWebhookServer(provider *TrafficManagerProvider, logger *zap.Logger) *WebhookServer {
	return &WebhookServer{
		provider: provider,
		logger:   logger,
	}
}

// HandleNegotiate handles GET / - Domain filter negotiation
func (s *WebhookServer) HandleNegotiate(w http.ResponseWriter, r *http.Request) {
	s.logger.Info("Handling negotiation request",
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
	)

	if r.Method != http.MethodGet {
		s.logger.Warn("Invalid method for negotiation", zap.String("method", r.Method))
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := NegotiationResponse{
		Version: "1",
		DomainFilter: DomainFilter{
			Include: s.provider.domainFilter,
			Exclude: []string{},
		},
	}

	w.Header().Set("Content-Type", "application/external.dns.webhook+json;version=1")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("Failed to encode negotiation response", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	s.logger.Info("Negotiation response sent successfully", zap.Any("domainFilter", s.provider.domainFilter))
}

// HandleHealth handles GET /healthz - Health check
func (s *WebhookServer) HandleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := HealthResponse{
		Status: "healthy",
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("Failed to encode health response", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// HandleRecords handles GET /records and POST /records
func (s *WebhookServer) HandleRecords(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleGetRecords(w, r)
	case http.MethodPost:
		s.handleApplyChanges(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGetRecords handles GET /records - Get current records
func (s *WebhookServer) handleGetRecords(w http.ResponseWriter, r *http.Request) {
	s.logger.Info("Handling get records request")

	endpoints, err := s.provider.Records(r.Context())
	if err != nil {
		s.logger.Error("Failed to get records", zap.Error(err))
		http.Error(w, fmt.Sprintf("Failed to get records: %v", err), http.StatusInternalServerError)
		return
	}

	// Return endpoints array directly, not wrapped in an object
	w.Header().Set("Content-Type", "application/external.dns.webhook+json;version=1")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(endpoints); err != nil {
		s.logger.Error("Failed to encode records response", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	s.logger.Info("Successfully returned records", zap.Int("count", len(endpoints)))
}

// handleApplyChanges handles POST /records - Apply changes
func (s *WebhookServer) handleApplyChanges(w http.ResponseWriter, r *http.Request) {
	s.logger.Info("Handling apply changes request")

	var changes Changes
	if err := json.NewDecoder(r.Body).Decode(&changes); err != nil {
		s.logger.Error("Failed to decode changes request", zap.Error(err))
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	s.logger.Info("Parsed changes",
		zap.Int("create", len(changes.Create)),
		zap.Int("updateOld", len(changes.UpdateOld)),
		zap.Int("updateNew", len(changes.UpdateNew)),
		zap.Int("delete", len(changes.Delete)))

	if err := s.provider.ApplyChanges(r.Context(), &changes); err != nil {
		s.logger.Error("Failed to apply changes", zap.Error(err))
		http.Error(w, fmt.Sprintf("Failed to apply changes: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
	s.logger.Info("Successfully applied changes")
}

// HandleAdjustEndpoints handles POST /adjustendpoints
func (s *WebhookServer) HandleAdjustEndpoints(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.logger.Info("Handling adjust endpoints request")

	// External-DNS sends endpoints array directly, not wrapped in an object
	var endpoints []*Endpoint
	if err := json.NewDecoder(r.Body).Decode(&endpoints); err != nil {
		s.logger.Error("Failed to decode adjust endpoints request", zap.Error(err))
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	s.logger.Info("Received endpoints to adjust", zap.Int("count", len(endpoints)))

	// Adjust endpoints with Traffic Manager annotations
	// Convert service A records to CNAME records pointing to Traffic Manager profiles
	adjustedEndpoints := s.provider.AdjustEndpoints(r.Context(), endpoints)
	
	w.Header().Set("Content-Type", "application/external.dns.webhook+json;version=1")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(adjustedEndpoints); err != nil {
		s.logger.Error("Failed to encode adjust endpoints response", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	s.logger.Info("Successfully adjusted endpoints", zap.Int("returned", len(adjustedEndpoints)))
}
