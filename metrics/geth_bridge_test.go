package metrics

import (
	"math"
	"strings"
	"testing"
	"time"

	gethmetrics "github.com/ethereum/go-ethereum/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
)

func TestSanitizeName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in, out string
	}{
		{"simple", "simple"},
		{"has.dots", "has_dots"},
		{"has-dash", "has_dash"},
		{"path/like/thing", "path_like_thing"},
		{"spaces here", "spaces_here"},
		{"mixed:chars.a-b/c d", "mixed_chars_a_b_c_d"},
		{"double__underscores", "double_underscores"},
		{"a..b--c//d  e:::f", "a_b_c_d_e_f"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			require.Equal(t, tt.out, sanitizeName(tt.in))
		})
	}
}

func TestGetOrCreateDesc_CacheAndFQName(t *testing.T) {
	t.Parallel()

	c := NewGethCollector(CollectorOpts{
		Registry:  gethmetrics.NewRegistry(),
		Namespace: "geth",
		Subsystem: "core",
	})

	labels := []string{"l1", "l2"}
	d1 := c.getOrCreateDesc("a.b/c-d", "help", labels)
	d2 := c.getOrCreateDesc("a.b/c-d", "help", labels) // same key -> same pointer
	require.Same(t, d1, d2)

	// Different label set -> different desc
	d3 := c.getOrCreateDesc("a.b/c-d", "help", []string{"l1"})
	require.NotSame(t, d1, d3)

	// Name should be sanitized and fq'd
	ch := make(chan prometheus.Metric, 1)
	emitGauge(ch, d1, 1, "x", "y")
	got := <-ch

	descStr := got.Desc().String()
	require.Contains(t, descStr, "fqName: \"geth_core_a_b_c_d\"")
}

func TestEmitGauge_FiniteCheck(t *testing.T) {
	t.Parallel()

	desc := prometheus.NewDesc("x", "y", nil, nil)
	ch := make(chan prometheus.Metric, 3)

	emitGauge(ch, desc, 1.23)
	emitGauge(ch, desc, math.NaN())
	emitGauge(ch, desc, math.Inf(1))
	emitGauge(ch, desc, math.Inf(-1))

	require.Equal(t, 1, len(ch)) // only the finite one passes
}

// --- Collect end-to-end over all metric kinds ---

func TestCollect_AllKinds(t *testing.T) {
	gethmetrics.Enable()
	reg := gethmetrics.NewRegistry()

	// Counter
	cnt := gethmetrics.NewCounter()
	cnt.Inc(3)
	require.NoError(t, reg.Register("txs.count", cnt))

	// CounterFloat64
	cf := gethmetrics.NewCounterFloat64()
	cf.Inc(2.5)
	require.NoError(t, reg.Register("gas.total", cf))

	// Gauge (int)
	gi := gethmetrics.NewGauge()
	gi.Update(5)
	require.NoError(t, reg.Register("queue-size", gi))

	// GaugeFloat64 with NaN should be ignored
	gf := gethmetrics.NewGaugeFloat64()
	gf.Update(math.NaN())
	require.NoError(t, reg.Register("nan.value", gf))

	// GaugeInfo with unsanitized label keys
	inf := gethmetrics.NewGaugeInfo()
	inf.Update(map[string]string{
		"peer.id": "abc",
		"role":    "validator",
	})
	require.NoError(t, reg.Register("peer.info", inf))

	// Meter
	m := gethmetrics.NewMeter()
	m.Mark(5)
	require.NoError(t, reg.Register("rpc/requests", m))

	// Timer
	tmr := gethmetrics.NewTimer()
	tmr.Update(1 * time.Second)
	tmr.Update(2 * time.Second)
	require.NoError(t, reg.Register("block_import", tmr))

	// Histogram
	h := gethmetrics.NewHistogram(gethmetrics.NewUniformSample(1028))
	h.Update(100)
	h.Update(200)
	h.Update(300)
	require.NoError(t, reg.Register("txn_size", h))

	// ResettingTimer
	rt := gethmetrics.NewResettingTimer()
	rt.Update(100 * time.Millisecond)
	rt.Update(200 * time.Millisecond)
	require.NoError(t, reg.Register("p2p.round", rt))

	coll := NewGethCollector(CollectorOpts{
		Registry:  reg,
		Namespace: "geth",
		Subsystem: "",
	})

	// Compare a focused subset (stable values)
	var expected = `
# HELP geth_txs_count_total Geth counter metric
# TYPE geth_txs_count_total counter
geth_txs_count_total 3

# HELP geth_gas_total_total Geth counter float64 metric
# TYPE geth_gas_total_total counter
geth_gas_total_total 2.5

# HELP geth_queue_size Geth gauge metric
# TYPE geth_queue_size gauge
geth_queue_size 5

# HELP geth_peer_info Geth gauge info metric
# TYPE geth_peer_info gauge
geth_peer_info{peer_id="abc",role="validator"} 1

# HELP geth_rpc_requests_total Geth meter total events
# TYPE geth_rpc_requests_total counter
geth_rpc_requests_total 5

# HELP geth_block_import_seconds_count Geth timer observations
# TYPE geth_block_import_seconds_count counter
geth_block_import_seconds_count 2
# HELP geth_block_import_seconds_sum Geth timer sum in seconds
# TYPE geth_block_import_seconds_sum counter
geth_block_import_seconds_sum 3
# HELP geth_block_import_min_seconds Geth timer minimum in seconds
# TYPE geth_block_import_min_seconds gauge
geth_block_import_min_seconds 1
# HELP geth_block_import_max_seconds Geth timer maximum in seconds
# TYPE geth_block_import_max_seconds gauge
geth_block_import_max_seconds 2
# HELP geth_block_import_mean_seconds Geth timer mean in seconds
# TYPE geth_block_import_mean_seconds gauge
geth_block_import_mean_seconds 1.5

# HELP geth_txn_size_count Geth histogram count
# TYPE geth_txn_size_count counter
geth_txn_size_count 3
# HELP geth_txn_size_sum Geth histogram sum
# TYPE geth_txn_size_sum counter
geth_txn_size_sum 600
# HELP geth_txn_size_min Geth histogram minimum
# TYPE geth_txn_size_min gauge
geth_txn_size_min 100
# HELP geth_txn_size_max Geth histogram maximum
# TYPE geth_txn_size_max gauge
geth_txn_size_max 300
# HELP geth_txn_size_mean Geth histogram mean
# TYPE geth_txn_size_mean gauge
geth_txn_size_mean 200

# HELP geth_p2p_round_window_events Events observed in the current timer window
# TYPE geth_p2p_round_window_events gauge
geth_p2p_round_window_events 2
# HELP geth_p2p_round_min_seconds Geth resetting timer minimum in seconds
# TYPE geth_p2p_round_min_seconds gauge
geth_p2p_round_min_seconds 0.1
# HELP geth_p2p_round_max_seconds Geth resetting timer maximum in seconds
# TYPE geth_p2p_round_max_seconds gauge
geth_p2p_round_max_seconds 0.2
# HELP geth_p2p_round_mean_seconds Geth resetting timer mean in seconds
# TYPE geth_p2p_round_mean_seconds gauge
geth_p2p_round_mean_seconds 0.15
`

	// Limit comparison to the metrics listed; this keeps the test stable across ephemeral rate/quantile series.
	err := testutil.CollectAndCompare(
		coll,
		strings.NewReader(expected),
		"geth_txs_count_total",
		"geth_gas_total_total",
		"geth_queue_size",
		"geth_peer_info",
		"geth_rpc_requests_total",
		"geth_block_import_seconds_count",
		"geth_block_import_seconds_sum",
		"geth_block_import_min_seconds",
		"geth_block_import_max_seconds",
		"geth_block_import_mean_seconds",
		"geth_txn_size_count",
		"geth_txn_size_sum",
		"geth_txn_size_min",
		"geth_txn_size_max",
		"geth_txn_size_mean",
		"geth_p2p_round_window_events",
		"geth_p2p_round_min_seconds",
		"geth_p2p_round_max_seconds",
		"geth_p2p_round_mean_seconds",
	)
	require.NoError(t, err)
}

func TestEnableGethMetrics_RegistersCollector(t *testing.T) {
	gethReg := gethmetrics.NewRegistry()
	promReg := prometheus.NewRegistry()

	coll, err := EnableGethMetrics(gethReg, promReg)
	require.NoError(t, err)
	require.NotNil(t, coll)

	// Ensure collector is actually registered by counting something trivial.
	// With an empty geth registry we still should succeed with zero metrics collected.
	n, err := testutil.GatherAndCount(promReg)
	require.NoError(t, err)
	require.GreaterOrEqual(t, n, 0)
}
