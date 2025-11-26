package backend

import "go.opentelemetry.io/otel"

var tracer = otel.Tracer("evm/rpc/backend")
