package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/PlainFunction/mistokenly/internal/common/config"
	"github.com/PlainFunction/mistokenly/internal/common/grpc"
	"github.com/PlainFunction/mistokenly/internal/common/types"
	pbAudit "github.com/PlainFunction/mistokenly/proto/audit"
	pb "github.com/PlainFunction/mistokenly/proto/pii"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Handler struct {
	config       *config.Config
	piiService   grpc.PIIServiceInterface
	registry     *grpc.ServiceRegistry
	auditService types.AuditServiceInterface

	// Prometheus metrics
	requestsTotal      *prometheus.CounterVec
	requestDuration    *prometheus.HistogramVec
	tokenizeRequests   prometheus.Counter
	detokenizeRequests prometheus.Counter
}

func NewHandler(cfg *config.Config) *Handler {
	registry := grpc.NewServiceRegistry(cfg)

	// Get PII service client
	piiService, err := registry.GetPIIServiceClient()
	if err != nil {
		panic("Failed to get PII service client: " + err.Error())
	}

	// Get Audit service client
	auditService, err := registry.GetAuditServiceClient()
	if err != nil {
		panic("Failed to get Audit service client: " + err.Error())
	}

	// Initialize Prometheus metrics
	requestsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_requests_total",
			Help: "Total number of API requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	requestDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "api_request_duration_seconds",
			Help:    "Duration of API requests",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)

	tokenizeRequests := prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "api_tokenize_requests_total",
			Help: "Total number of tokenize requests",
		},
	)

	detokenizeRequests := prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "api_detokenize_requests_total",
			Help: "Total number of detokenize requests",
		},
	)

	// Register metrics
	prometheus.MustRegister(requestsTotal, requestDuration, tokenizeRequests, detokenizeRequests)

	return &Handler{
		config:             cfg,
		piiService:         piiService,
		registry:           registry,
		auditService:       auditService,
		requestsTotal:      requestsTotal,
		requestDuration:    requestDuration,
		tokenizeRequests:   tokenizeRequests,
		detokenizeRequests: detokenizeRequests,
	}
}

func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	req := &pb.HealthCheckRequest{
		ServiceName: "api-gateway",
	}

	ctx := r.Context()
	resp, err := h.piiService.HealthCheck(ctx, req)
	if err != nil {
		h.requestsTotal.WithLabelValues("GET", "/health", "500").Inc()
		h.requestDuration.WithLabelValues("GET", "/health").Observe(time.Since(start).Seconds())
		errorResp := map[string]interface{}{
			"error":   "internal_server_error",
			"code":    "HEALTH_CHECK_FAILED",
			"message": fmt.Sprintf("Health check failed: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errorResp)
		return
	}

	h.requestsTotal.WithLabelValues("GET", "/health", "200").Inc()
	h.requestDuration.WithLabelValues("GET", "/health").Observe(time.Since(start).Seconds())

	// Convert protobuf to JSON
	jsonBytes, err := protojson.Marshal(resp)
	if err != nil {
		h.requestsTotal.WithLabelValues("GET", "/health", "500").Inc()
		h.requestDuration.WithLabelValues("GET", "/health").Observe(time.Since(start).Seconds())
		errorResp := map[string]interface{}{
			"error":   "internal_server_error",
			"code":    "MARSHAL_FAILED",
			"message": fmt.Sprintf("Failed to marshal response: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errorResp)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonBytes)
}

func (h *Handler) Tokenize(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Read request body and unmarshal JSON directly into protobuf message
	req := &pb.TokenizeRequest{}

	decoder := json.NewDecoder(r.Body)
	var jsonReq map[string]interface{}
	if err := decoder.Decode(&jsonReq); err != nil {
		h.requestsTotal.WithLabelValues("POST", "/tokenize", "400").Inc()
		h.requestDuration.WithLabelValues("POST", "/tokenize").Observe(time.Since(start).Seconds())
		errorResp := map[string]interface{}{
			"error":   "bad_request",
			"code":    "INVALID_REQUEST_BODY",
			"message": "Invalid request body",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errorResp)
		return
	}

	// Manually map JSON fields to protobuf (handles camelCase to snake_case)
	if data, ok := jsonReq["data"].(string); ok {
		req.Data = data
	}
	if dataType, ok := jsonReq["dataType"].(string); ok {
		req.DataType = dataType
	}
	if retentionPolicy, ok := jsonReq["retentionPolicy"].(string); ok {
		req.RetentionPolicy = retentionPolicy
	}
	if clientId, ok := jsonReq["clientId"].(string); ok {
		req.ClientId = clientId
	}
	if organizationId, ok := jsonReq["organizationId"].(string); ok {
		req.OrganizationId = organizationId
	}
	if organizationKey, ok := jsonReq["organizationKey"].(string); ok {
		req.OrganizationKey = organizationKey
	}
	if metadata, ok := jsonReq["metadata"].(map[string]interface{}); ok {
		req.Metadata = make(map[string]string)
		for k, v := range metadata {
			if str, ok := v.(string); ok {
				req.Metadata[k] = str
			}
		}
	}

	ctx := r.Context()
	resp, err := h.piiService.Tokenize(ctx, req)
	if err != nil {
		h.requestsTotal.WithLabelValues("POST", "/tokenize", "500").Inc()
		h.requestDuration.WithLabelValues("POST", "/tokenize").Observe(time.Since(start).Seconds())
		errorResp := map[string]interface{}{
			"error":   "internal_server_error",
			"code":    "TOKENIZE_FAILED",
			"message": fmt.Sprintf("Tokenize failed: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errorResp)
		return
	}

	// Check for application-level errors
	if resp.Status == "error" {
		h.requestsTotal.WithLabelValues("POST", "/tokenize", "400").Inc()
		h.requestDuration.WithLabelValues("POST", "/tokenize").Observe(time.Since(start).Seconds())
		errorResp := map[string]string{
			"error":   "error",
			"code":    "TOKENIZE_ERROR",
			"message": resp.ErrorMessage,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errorResp)
		return
	}

	// Convert protobuf to JSON
	jsonBytes, err := protojson.Marshal(resp)
	if err != nil {
		h.requestsTotal.WithLabelValues("POST", "/tokenize", "500").Inc()
		h.requestDuration.WithLabelValues("POST", "/tokenize").Observe(time.Since(start).Seconds())
		errorResp := map[string]interface{}{
			"error":   "internal_server_error",
			"code":    "MARSHAL_FAILED",
			"message": fmt.Sprintf("Failed to marshal response: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errorResp)
		return
	}

	h.tokenizeRequests.Inc()
	h.requestsTotal.WithLabelValues("POST", "/tokenize", "200").Inc()
	h.requestDuration.WithLabelValues("POST", "/tokenize").Observe(time.Since(start).Seconds())

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonBytes)

	// Audit log for tokenization
	auditReq := &pbAudit.LogAccessRequest{
		ReferenceHash:     resp.ReferenceHash,
		Operation:         "tokenize",
		RequestingService: "api-gateway",
		RequestingUser:    req.ClientId,
		Purpose:           req.RetentionPolicy,
		Timestamp:         timestamppb.New(time.Now()),
		ClientIp:          r.RemoteAddr,
		Metadata:          req.Metadata,
	}
	h.auditService.LogAccess(ctx, auditReq)
}

func (h *Handler) Detokenize(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Unmarshal JSON directly into protobuf message
	req := &pb.DetokenizeRequest{}
	decoder := json.NewDecoder(r.Body)
	var jsonReq map[string]interface{}
	if err := decoder.Decode(&jsonReq); err != nil {
		h.requestsTotal.WithLabelValues("POST", "/detokenize", "400").Inc()
		h.requestDuration.WithLabelValues("POST", "/detokenize").Observe(time.Since(start).Seconds())
		errorResp := map[string]interface{}{
			"error":   "bad_request",
			"code":    "INVALID_REQUEST_BODY",
			"message": "Invalid request body",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errorResp)
		return
	}

	// Manually map JSON fields to protobuf (handles camelCase to snake_case)
	if referenceHash, ok := jsonReq["referenceHash"].(string); ok {
		req.ReferenceHash = referenceHash
	}
	if purpose, ok := jsonReq["purpose"].(string); ok {
		req.Purpose = purpose
	}
	if requestingService, ok := jsonReq["requestingService"].(string); ok {
		req.RequestingService = requestingService
	}
	if requestingUser, ok := jsonReq["requestingUser"].(string); ok {
		req.RequestingUser = requestingUser
	}
	if organizationId, ok := jsonReq["organizationId"].(string); ok {
		req.OrganizationId = organizationId
	}
	if organizationKey, ok := jsonReq["organizationKey"].(string); ok {
		req.OrganizationKey = organizationKey
	}

	ctx := r.Context()
	resp, err := h.piiService.Detokenize(ctx, req)
	if err != nil {
		h.requestsTotal.WithLabelValues("POST", "/detokenize", "500").Inc()
		h.requestDuration.WithLabelValues("POST", "/detokenize").Observe(time.Since(start).Seconds())
		errorResp := map[string]interface{}{
			"error":   "internal_server_error",
			"code":    "DETOKENIZE_FAILED",
			"message": fmt.Sprintf("Detokenize failed: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errorResp)
		return
	}

	// Check for application-level errors
	if resp.Status == "error" {
		h.requestsTotal.WithLabelValues("POST", "/detokenize", "400").Inc()
		h.requestDuration.WithLabelValues("POST", "/detokenize").Observe(time.Since(start).Seconds())
		errorResp := map[string]string{
			"error":   "error",
			"code":    "DETOKENIZE_ERROR",
			"message": resp.ErrorMessage,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errorResp)
		return
	}

	// Convert protobuf to JSON
	jsonBytes, err := protojson.Marshal(resp)
	if err != nil {
		h.requestsTotal.WithLabelValues("POST", "/detokenize", "500").Inc()
		h.requestDuration.WithLabelValues("POST", "/detokenize").Observe(time.Since(start).Seconds())
		errorResp := map[string]interface{}{
			"error":   "internal_server_error",
			"code":    "MARSHAL_FAILED",
			"message": fmt.Sprintf("Failed to marshal response: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errorResp)
		return
	}

	h.detokenizeRequests.Inc()
	h.requestsTotal.WithLabelValues("POST", "/detokenize", "200").Inc()
	h.requestDuration.WithLabelValues("POST", "/detokenize").Observe(time.Since(start).Seconds())

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonBytes)

	// Audit log for detokenization
	auditReq := &pbAudit.LogAccessRequest{
		ReferenceHash:     req.ReferenceHash,
		Operation:         "detokenize",
		RequestingService: "api-gateway",
		RequestingUser:    req.RequestingUser,
		Purpose:           req.Purpose,
		Timestamp:         timestamppb.New(time.Now()),
		ClientIp:          r.RemoteAddr,
		Metadata:          nil,
	}
	h.auditService.LogAccess(ctx, auditReq)
}

func (h *Handler) Metrics(w http.ResponseWriter, r *http.Request) {
	promhttp.Handler().ServeHTTP(w, r)
}

func (h *Handler) GetAuditLogs(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Parse query parameters
	req := &pbAudit.GetAuditLogsRequest{}

	// Parse time range
	if startTimeStr := r.URL.Query().Get("startTime"); startTimeStr != "" {
		if startTime, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
			req.StartTime = timestamppb.New(startTime)
		}
	}
	if endTimeStr := r.URL.Query().Get("endTime"); endTimeStr != "" {
		if endTime, err := time.Parse(time.RFC3339, endTimeStr); err == nil {
			req.EndTime = timestamppb.New(endTime)
		}
	}

	// Parse other filters
	req.ReferenceHash = r.URL.Query().Get("referenceHash")
	req.RequestingService = r.URL.Query().Get("requestingService")

	// Parse limit and offset
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			req.Limit = int32(limit)
		}
	}
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			req.Offset = int32(offset)
		}
	}

	ctx := r.Context()
	resp, err := h.auditService.GetAuditLogs(ctx, req)
	if err != nil {
		h.requestsTotal.WithLabelValues("GET", "/audit/logs", "500").Inc()
		h.requestDuration.WithLabelValues("GET", "/audit/logs").Observe(time.Since(start).Seconds())
		errorResp := map[string]interface{}{
			"error":   "internal_server_error",
			"code":    "GET_AUDIT_LOGS_FAILED",
			"message": fmt.Sprintf("GetAuditLogs failed: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errorResp)
		return
	}

	// Convert to JSON
	jsonBytes, err := json.Marshal(resp)
	if err != nil {
		h.requestsTotal.WithLabelValues("GET", "/audit/logs", "500").Inc()
		h.requestDuration.WithLabelValues("GET", "/audit/logs").Observe(time.Since(start).Seconds())
		errorResp := map[string]interface{}{
			"error":   "internal_server_error",
			"code":    "MARSHAL_FAILED",
			"message": fmt.Sprintf("Failed to marshal response: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errorResp)
		return
	}

	h.requestsTotal.WithLabelValues("GET", "/audit/logs", "200").Inc()
	h.requestDuration.WithLabelValues("GET", "/audit/logs").Observe(time.Since(start).Seconds())

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonBytes)
}
