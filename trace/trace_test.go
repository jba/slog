package trace

import (
	"context"
	"log/slog"
	"testing"
)

func Test(t *testing.T) {
	tr := &Tracer{}

	ctx := context.Background()
	ctx, s := tr.Start(ctx, "main")
	defer s.End()
	logger := slog.New(&handler{slog.Default().Handler()})
	logger.InfoContext(ctx, "in main")
	f(ctx, tr, logger)
	logger.InfoContext(ctx, "still in main")
}

func f(ctx context.Context, tr *Tracer, l *slog.Logger) {
	ctx, s := tr.Start(ctx, "f")
	defer s.End()
	l.InfoContext(ctx, "in f")
	g(ctx, tr, l)
	l.InfoContext(ctx, "still in f")
}

func g(ctx context.Context, tr *Tracer, l *slog.Logger) {
	ctx, s := tr.Start(ctx, "g")
	defer s.End()
	l.InfoContext(ctx, "in g")
}

type handler struct {
	slog.Handler
}

func (h *handler) Handle(ctx context.Context, r slog.Record) error {
	if name := SpanName(ctx); name != "" {
		r.AddAttrs(slog.String("span", name))
	}
	return h.Handler.Handle(ctx, r)
}
