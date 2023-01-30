package trace

import (
	"context"
	"sync"

	otrace "go.opentelemetry.io/otel/trace"
)

type spanListKey struct{}

// A spanList is a sequence of spans, safe for concurrent use.
type spanList struct {
	mu    sync.Mutex
	spans []*span
}

func (sl *spanList) append(s *span) {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	sl.spans = append(sl.spans, s)
	s.list = sl
}

func (sl *spanList) remove(s *span) {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	n := len(sl.spans)
	if n > 0 && sl.spans[n-1] == s {
		sl.spans = sl.spans[:n-1]
	} else {
		// TODO: remove s from the middle?
	}
}

type Tracer struct {
}

var _ otrace.Tracer = (*Tracer)(nil)

func (t *Tracer) Start(ctx context.Context, name string, opts ...otrace.SpanStartOption) (context.Context, otrace.Span) {
	s := &span{name: name}
	// Append the new span to the context's spanList, adding a spanList if there is none.
	sl, ok := ctx.Value(spanListKey{}).(*spanList)
	if !ok {
		sl = &spanList{}
		ctx = context.WithValue(ctx, spanListKey{}, sl)
	}
	sl.append(s)
	return otrace.ContextWithSpan(ctx, s), s
}

type span struct {
	otrace.Span
	name string
	list *spanList
}

func (s *span) End(options ...otrace.SpanEndOption) {
	// Remove the span from the context's spanList.
	s.list.remove(s)
}

// for testing
func (s *span) Name() string {
	return s.name
}

func SpanName(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	sl, ok := ctx.Value(spanListKey{}).(*spanList)
	if !ok || len(sl.spans) == 0 {
		return ""
	}
	return sl.spans[len(sl.spans)-1].Name()
}
