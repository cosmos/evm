package legacypool

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

type GethCollector struct {
	registry metrics.Registry
	descs    map[string]*prometheus.Desc
	mu       sync.RWMutex

	// configurable
	namespace string
	subsystem string
}

// CollectorOpts configures the GethCollector
type CollectorOpts struct {
	Registry  metrics.Registry
	Namespace string
	Subsystem string
}

// NewGethCollector creates a new collector with options
func NewGethCollector(opts CollectorOpts) *GethCollector {
	if opts.Registry == nil {
		opts.Registry = metrics.DefaultRegistry
	}
	if opts.Namespace == "" {
		opts.Namespace = "geth"
	}

	return &GethCollector{
		registry:  opts.Registry,
		descs:     make(map[string]*prometheus.Desc),
		namespace: opts.Namespace,
		subsystem: opts.Subsystem,
	}
}

// sanitizeName ensures the metric name is Prometheus-compliant
func sanitizeName(name string) string {
	// Replace common invalid characters with underscores
	replacer := strings.NewReplacer(
		".", "_",
		"-", "_",
		"/", "_",
		" ", "_",
		":", "_",
	)
	s := replacer.Replace(name)
	// (Optional) collapse double underscores — safe to leave as-is if not desired
	for strings.Contains(s, "__") {
		s = strings.ReplaceAll(s, "__", "_")
	}
	return s
}

func (g *GethCollector) getOrCreateDesc(name, help string, labels []string) *prometheus.Desc {
	// Create a unique key that includes labels to avoid conflicts
	key := name
	if len(labels) > 0 {
		key = fmt.Sprintf("%s|%s", name, strings.Join(labels, ","))
	}

	g.mu.RLock()
	desc, exists := g.descs[key]
	g.mu.RUnlock()
	if exists {
		return desc
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	// double-check after acquiring write lock
	if desc, exists = g.descs[key]; exists {
		return desc
	}

	desc = prometheus.NewDesc(
		prometheus.BuildFQName(g.namespace, g.subsystem, sanitizeName(name)),
		help,
		labels,
		nil,
	)
	g.descs[key] = desc
	return desc
}

func (g *GethCollector) Describe(descs chan<- *prometheus.Desc) {
	// Intentionally empty: unchecked collector pattern.
	// (Optionally: prometheus.DescribeByCollect(descs, g) to enable static checks.)
}

func (g *GethCollector) Collect(ch chan<- prometheus.Metric) {
	g.registry.Each(func(name string, metric interface{}) {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("Error collecting metric %s: %v\n", name, r)
			}
		}()

		// CRITICAL: Use pointer types and match Geth's actual types
		switch m := metric.(type) {
		case *metrics.Counter:
			g.collectCounter(ch, name, m.Snapshot())
		case *metrics.CounterFloat64:
			g.collectCounterFloat64(ch, name, m.Snapshot())
		case *metrics.Gauge:
			g.collectGauge(ch, name, m.Snapshot())
		case *metrics.GaugeFloat64:
			g.collectGaugeFloat64(ch, name, m.Snapshot())
		case *metrics.GaugeInfo:
			g.collectGaugeInfo(ch, name, m.Snapshot())
		case metrics.Histogram: // interface, pointer not needed
			g.collectHistogram(ch, name, m.Snapshot())
		case *metrics.Meter:
			g.collectMeter(ch, name, m.Snapshot())
		case *metrics.Timer:
			g.collectTimer(ch, name, m.Snapshot())
		case *metrics.ResettingTimer:
			g.collectResettingTimer(ch, name, m.Snapshot())
		default:
			// Debug: log unknown types
			fmt.Printf("Unknown metric type for %s: %T\n", name, metric)
		}
	})
}

// --- helpers ---

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

func emitGauge(ch chan<- prometheus.Metric, desc *prometheus.Desc, v float64, labelValues ...string) {
	if !isFinite(v) {
		return
	}
	if m, err := prometheus.NewConstMetric(desc, prometheus.GaugeValue, v, labelValues...); err == nil {
		ch <- m
	}
}

// --- collectors ---

func (g *GethCollector) collectMeter(ch chan<- prometheus.Metric, name string, snapshot *metrics.MeterSnapshot) {
	// total events (monotonic counter)
	totalDesc := g.getOrCreateDesc(name+"_total", "Geth meter total events", nil)
	if m, err := prometheus.NewConstMetric(totalDesc, prometheus.CounterValue, float64(snapshot.Count())); err == nil {
		ch <- m
	}

	// EWMA / mean rates (events per second) as gauges
	for _, r := range []struct {
		suffix string
		value  float64
		help   string
	}{
		{"_rate_1m", snapshot.Rate1(), "Geth meter 1-minute rate (events per second)"},
		{"_rate_5m", snapshot.Rate5(), "Geth meter 5-minute rate (events per second)"},
		{"_rate_15m", snapshot.Rate15(), "Geth meter 15-minute rate (events per second)"},
		{"_rate_mean", snapshot.RateMean(), "Geth meter mean rate (events per second)"},
	} {
		desc := g.getOrCreateDesc(name+r.suffix, r.help, nil)
		emitGauge(ch, desc, r.value)
	}
}

func (g *GethCollector) collectTimer(ch chan<- prometheus.Metric, name string, snapshot *metrics.TimerSnapshot) {
	// Treat Timer as summary-like with a unitful base name
	// Base: <name>_seconds{quantile="..."} as gauges
	// Sum/Count: <name>_seconds_sum / <name>_seconds_count as counters

	// counters
	countDesc := g.getOrCreateDesc(name+"_seconds_count", "Geth timer observations", nil)
	if m, err := prometheus.NewConstMetric(countDesc, prometheus.CounterValue, float64(snapshot.Count())); err == nil {
		ch <- m
	}
	sumDesc := g.getOrCreateDesc(name+"_seconds_sum", "Geth timer sum in seconds", nil)
	if m, err := prometheus.NewConstMetric(sumDesc, prometheus.CounterValue, float64(snapshot.Sum())/1e9); err == nil {
		ch <- m
	}

	// quantiles
	qdesc := g.getOrCreateDesc(name+"_seconds", "Geth timer percentile in seconds", []string{"quantile"})
	qs := []float64{0.5, 0.75, 0.95, 0.99, 0.999, 0.9999}
	qvals := snapshot.Percentiles(qs)
	for i, qp := range qvals {
		v := float64(qp) / 1e9
		emitGauge(ch, qdesc, v, strconv.FormatFloat(qs[i], 'f', -1, 64))
	}

	// min / max / mean (seconds)
	emitGauge(ch, g.getOrCreateDesc(name+"_min_seconds", "Geth timer minimum in seconds", nil), float64(snapshot.Min())/1e9)
	emitGauge(ch, g.getOrCreateDesc(name+"_max_seconds", "Geth timer maximum in seconds", nil), float64(snapshot.Max())/1e9)
	emitGauge(ch, g.getOrCreateDesc(name+"_mean_seconds", "Geth timer mean in seconds", nil), snapshot.Mean()/1e9)

	// rates (events per second)
	for _, r := range []struct {
		suffix string
		value  float64
		help   string
	}{
		{"_rate_1m", snapshot.Rate1(), "Geth timer 1-minute rate (events per second)"},
		{"_rate_5m", snapshot.Rate5(), "Geth timer 5-minute rate (events per second)"},
		{"_rate_15m", snapshot.Rate15(), "Geth timer 15-minute rate (events per second)"},
		{"_rate_mean", snapshot.RateMean(), "Geth timer mean rate (events per second)"},
	} {
		desc := g.getOrCreateDesc(name+r.suffix, r.help, nil)
		emitGauge(ch, desc, r.value)
	}
}

func (g *GethCollector) collectHistogram(ch chan<- prometheus.Metric, name string, snapshot metrics.HistogramSnapshot) {
	// Expose as summary-like: quantiles + _sum/_count
	// If you want true histograms, you'd need to predefine buckets and emit _bucket series.

	// counters
	countDesc := g.getOrCreateDesc(name+"_count", "Geth histogram count", nil)
	if m, err := prometheus.NewConstMetric(countDesc, prometheus.CounterValue, float64(snapshot.Count())); err == nil {
		ch <- m
	}
	sumDesc := g.getOrCreateDesc(name+"_sum", "Geth histogram sum", nil)
	if m, err := prometheus.NewConstMetric(sumDesc, prometheus.CounterValue, float64(snapshot.Sum())); err == nil {
		ch <- m
	}

	// quantiles (gauges). Add unit to base if appropriate for your data.
	qdesc := g.getOrCreateDesc(name, "Geth histogram percentile", []string{"quantile"})
	qs := []float64{0.5, 0.75, 0.95, 0.99, 0.999, 0.9999}
	qvals := snapshot.Percentiles(qs)
	for i, qp := range qvals {
		v := float64(qp)
		emitGauge(ch, qdesc, v, strconv.FormatFloat(qs[i], 'f', -1, 64))
	}

	// min/max/mean/stddev/variance as gauges
	emitGauge(ch, g.getOrCreateDesc(name+"_min", "Geth histogram minimum", nil), float64(snapshot.Min()))
	emitGauge(ch, g.getOrCreateDesc(name+"_max", "Geth histogram maximum", nil), float64(snapshot.Max()))
	emitGauge(ch, g.getOrCreateDesc(name+"_mean", "Geth histogram mean", nil), snapshot.Mean())
	emitGauge(ch, g.getOrCreateDesc(name+"_stddev", "Geth histogram standard deviation", nil), snapshot.StdDev())
	emitGauge(ch, g.getOrCreateDesc(name+"_variance", "Geth histogram variance", nil), snapshot.Variance())
}

func (g *GethCollector) collectResettingTimer(ch chan<- prometheus.Metric, name string, snapshot *metrics.ResettingTimerSnapshot) {
	// Skip empty timers like Geth does
	if snapshot.Count() <= 0 {
		return
	}

	// For resetting timer we expose current-window stats as gauges (no cumulative sum)
	countDesc := g.getOrCreateDesc(name+"_count", "Geth resetting timer count (current window)", nil)
	if m, err := prometheus.NewConstMetric(countDesc, prometheus.CounterValue, float64(snapshot.Count())); err == nil {
		ch <- m
	}

	// percentiles (seconds)
	qdesc := g.getOrCreateDesc(name+"_seconds", "Geth resetting timer percentile in seconds", []string{"quantile"})
	qs := []float64{0.5, 0.75, 0.95, 0.99, 0.999, 0.9999}
	qvals := snapshot.Percentiles(qs)
	for i, qp := range qvals {
		v := float64(qp) / 1e9
		emitGauge(ch, qdesc, v, strconv.FormatFloat(qs[i], 'f', -1, 64))
	}

	// min/max/mean (seconds)
	emitGauge(ch, g.getOrCreateDesc(name+"_min_seconds", "Geth resetting timer minimum in seconds", nil), float64(snapshot.Min())/1e9)
	emitGauge(ch, g.getOrCreateDesc(name+"_max_seconds", "Geth resetting timer maximum in seconds", nil), float64(snapshot.Max())/1e9)
	emitGauge(ch, g.getOrCreateDesc(name+"_mean_seconds", "Geth resetting timer mean in seconds", nil), snapshot.Mean()/1e9)
}

func (g *GethCollector) collectCounter(ch chan<- prometheus.Metric, name string, snapshot metrics.CounterSnapshot) {
	desc := g.getOrCreateDesc(name+"_total", "Geth counter metric", nil)
	if m, err := prometheus.NewConstMetric(desc, prometheus.CounterValue, float64(snapshot.Count())); err == nil {
		ch <- m
	}
}

func (g *GethCollector) collectCounterFloat64(ch chan<- prometheus.Metric, name string, snapshot metrics.CounterFloat64Snapshot) {
	desc := g.getOrCreateDesc(name+"_total", "Geth counter float64 metric", nil)
	if m, err := prometheus.NewConstMetric(desc, prometheus.CounterValue, snapshot.Count()); err == nil {
		ch <- m
	}
}

func (g *GethCollector) collectGauge(ch chan<- prometheus.Metric, name string, snapshot metrics.GaugeSnapshot) {
	v := float64(snapshot.Value())
	desc := g.getOrCreateDesc(name, "Geth gauge metric", nil)
	emitGauge(ch, desc, v)
}

func (g *GethCollector) collectGaugeFloat64(ch chan<- prometheus.Metric, name string, snapshot metrics.GaugeFloat64Snapshot) {
	v := snapshot.Value()
	desc := g.getOrCreateDesc(name, "Geth gauge float64 metric", nil)
	emitGauge(ch, desc, v)
}

func (g *GethCollector) collectGaugeInfo(ch chan<- prometheus.Metric, name string, snapshot metrics.GaugeInfoSnapshot) {
	// WARNING: dynamic label keys can increase cardinality drastically.
	keys := slices.Sorted(maps.Keys(snapshot.Value()))
	labels := make([]string, 0, len(keys))
	labelValues := make([]string, 0, len(keys))
	for _, k := range keys {
		labels = append(labels, sanitizeName(k))
		labelValues = append(labelValues, snapshot.Value()[k])
	}

	desc := g.getOrCreateDesc(name, "Geth gauge info metric", labels)
	// emit a constant gauge "1" carrying the info as labels
	if m, err := prometheus.NewConstMetric(desc, prometheus.GaugeValue, 1, labelValues...); err == nil {
		ch <- m
	}
}

var _ prometheus.Collector = &GethCollector{}

// RegisterCollector manually registers a GethCollector with the given registries.
// Prefer explicit registration over init() to avoid surprises in libraries/tests.
func RegisterCollector(metricsRegistry metrics.Registry, promRegistry prometheus.Registerer, opts ...CollectorOpts) (prometheus.Collector, error) {
	var o CollectorOpts
	if len(opts) > 0 {
		o = opts[0]
	}
	if o.Registry == nil {
		o.Registry = metricsRegistry
	}
	collector := NewGethCollector(CollectorOpts{
		Registry:  o.Registry,
		Namespace: o.Namespace,
		Subsystem: o.Subsystem,
	})
	return collector, promRegistry.Register(collector)
}

func init() {
	RegisterCollector(metrics.DefaultRegistry, prometheus.DefaultRegisterer, CollectorOpts{
		Namespace: "geth",
	})
}
