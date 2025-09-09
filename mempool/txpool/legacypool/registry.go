package legacypool

import (
	"net/http"

	gethmetrics "github.com/ethereum/go-ethereum/metrics"
	gethprom "github.com/ethereum/go-ethereum/metrics/prometheus"
)

func init() {
	// if you’re already using geth’s DefaultRegistry:
	http.Handle("/geth/metrics", gethprom.Handler(gethmetrics.DefaultRegistry))
	go func() {
		if err := http.ListenAndServe(":2112", nil); err != nil {
			panic(err)
		}
	}()
}
