package metrics

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	gethmetrics "github.com/ethereum/go-ethereum/metrics"
	gethprom "github.com/ethereum/go-ethereum/metrics/prometheus"
)

// StartGethMetricServer starts the geth metrics server on the specified port.
func StartGethMetricServer(ctx context.Context, addr string) {
	// Create a custom mux instead of using the global default
	mux := http.NewServeMux()
	mux.Handle("/metrics", gethprom.Handler(gethmetrics.DefaultRegistry))

	// Create server with custom mux
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Start server in goroutine
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("Metrics server error: %v", err)
		}
	}()

	// Respect context cancellation
	go func() {
		<-ctx.Done()
		log.Println("Shutting down metrics server...")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("Metrics server shutdown error: %v", err)
		}
	}()
}
