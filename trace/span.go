package trace

import (
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// EndSpanErr records the error to the span if not nil, and then ends the span.
// NOTE: This should be called like so:
//
// defer func() { EndSpanErr(span, err) }
//
// and not like this:
//
// defer EndSpanErr(span, err)
//
// This is due to the fact that the latter captures the error value at the point in time of the defer line,
// whilst the former will capture the final error value at the very end of the function.
func EndSpanErr(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}
