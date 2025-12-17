package observability

import (
	"context"
	"net/http"
	"sync"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// HealthChecker manages health checks for both gRPC and HTTP
type HealthChecker struct {
	grpcHealth *health.Server
	httpServer *http.Server
	logger     *zap.Logger
	mu         sync.RWMutex
	ready      bool
	kafkaReady bool
	usesKafka  bool
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(logger *zap.Logger) *HealthChecker {
	return &HealthChecker{
		grpcHealth: health.NewServer(),
		logger:     logger,
		ready:      true,
	}
}

// RegisterGRPC registers the health service with the gRPC server
func (h *HealthChecker) RegisterGRPC(s *grpc.Server) {
	grpc_health_v1.RegisterHealthServer(s, h.grpcHealth)
	h.grpcHealth.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
}

// StartHTTPServer starts the HTTP health check server
func (h *HealthChecker) StartHTTPServer(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", h.handleHealthz)

	h.httpServer = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	h.logger.Info("starting HTTP health server", zap.String("addr", addr))
	return h.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the health checker
func (h *HealthChecker) Shutdown(ctx context.Context) error {
	h.mu.Lock()
	h.ready = false
	h.grpcHealth.SetServingStatus("", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
	h.mu.Unlock()

	if h.httpServer != nil {
		return h.httpServer.Shutdown(ctx)
	}
	return nil
}

// SetKafkaReady sets the Kafka client readiness status
func (h *HealthChecker) SetKafkaReady(ready bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.kafkaReady = ready
	h.usesKafka = true
}

func (h *HealthChecker) handleHealthz(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	ready := h.ready
	kafkaReady := h.kafkaReady
	usesKafka := h.usesKafka
	h.mu.RUnlock()

	// Health check passes if ready is true and (not using Kafka or Kafka is ready)
	if ready && (!usesKafka || kafkaReady) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("NOT_READY"))
	}
}
