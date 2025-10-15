package metrics

import (
	"context"
	"errors"
	"net/http"
	"time"

	gethmetrics "github.com/ethereum/go-ethereum/metrics"
	gethprom "github.com/ethereum/go-ethereum/metrics/prometheus"

	"cosmossdk.io/log"
)

// StartGethMetricServer starts the geth metrics server on the specified address.
func StartGethMetricServer(ctx context.Context, log log.Logger, addr string) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", gethprom.Handler(gethmetrics.DefaultRegistry))

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)

	go func() {
		log.Info("starting geth metrics server...", "address", addr)
		errCh <- server.ListenAndServe()
	}()

	// Start a blocking select to wait for an indication to stop the server or that
	// the server failed to start properly.
	select {
	case <-ctx.Done():
		// The calling process canceled or closed the provided context, so we must
		// gracefully stop the metrics server.
		log.Info("stopping geth metrics server...", "address", addr)

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Error("geth metrics server shutdown error", "err", err)
			return err
		}
		return nil

	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("failed to start geth metrics server", "err", err)
			return err
		}
		return nil
	}
}
