package legacypool

import (
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

type GethCollector struct {
	registry metrics.Registry
}

func (g GethCollector) Describe(descs chan<- *prometheus.Desc) {
	//TODO implement me
	panic("implement me")
}

func (g GethCollector) Collect(c chan<- prometheus.Metric) {
	//TODO implement me
	panic("implement me")
}

var _ prometheus.Collector = GethCollector{}

func NewGethCollector(registry metrics.Registry) *GethCollector {
	return &GethCollector{registry: registry}
}

func init() {
	registry := metrics.DefaultRegistry
	reg := prometheus.DefaultRegisterer
	reg.Register()
}
