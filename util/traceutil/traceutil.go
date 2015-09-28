// Package traceutil contains appdash-related utilities.
package traceutil

import (
	"net/http"

	gcontext "github.com/gorilla/context"
	"golang.org/x/net/context"

	"sourcegraph.com/sourcegraph/appdash"
	"sourcegraph.com/sourcegraph/sourcegraph/util/traceutil/appdashctx"
)

// Recorder creates a new appdash Recorder for an existing span.
func Recorder(ctx context.Context) *appdash.Recorder {
	c := appdashctx.Collector(ctx)
	if c == nil {
		c = discardCollector{}
	}

	span := SpanIDFromContext(ctx)
	if span.Trace == 0 {
		// log.Println("no trace set in context")
	}
	return appdash.NewRecorder(span, c)
}

// DefaultCollector is the default Appdash collector to use. It is
// legacy and should not be used for additional things beyond the
// existing uses.
//
// TODO(sqs): remove this and make callers fetch the collector from
// the context instead of using a global here.
var DefaultCollector appdash.Collector

type discardCollector struct{}

func (discardCollector) Collect(appdash.SpanID, ...appdash.Annotation) error {
	return nil
}

type key int

const (
	spanID key = iota
)

// SpanID returns the Appdash span ID for the current HTTP request.
func SpanID(r *http.Request) appdash.SpanID {
	if v := gcontext.Get(r, spanID); v != nil {
		return v.(appdash.SpanID)
	}
	return appdash.SpanID{}
}

func SetSpanID(r *http.Request, v appdash.SpanID) {
	gcontext.Set(r, spanID, v)
}
