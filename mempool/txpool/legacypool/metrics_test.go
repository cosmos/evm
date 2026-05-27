package legacypool

import (
	"context"
	"os"
	"testing"

	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

var testMetricReader *sdkmetric.ManualReader

func TestMain(m *testing.M) {
	testMetricReader = sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(testMetricReader))
	otel.SetMeterProvider(provider)
	os.Exit(m.Run())
}

// readCounterTotal returns the accumulated value for a named int64 sum
// (counter) instrument, summing across all data points. Returns 0 if the
// instrument has not recorded any measurement yet.
func readCounterTotal(t *testing.T, name string) int64 {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := testMetricReader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("failed to collect metrics: %v", err)
	}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}
			sum, ok := m.Data.(metricdata.Sum[int64])
			if !ok {
				continue
			}
			var total int64
			for _, dp := range sum.DataPoints {
				total += dp.Value
			}
			return total
		}
	}
	return 0
}
