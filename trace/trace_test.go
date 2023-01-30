package trace

import (
	"context"
	"testing"

	"golang.org/x/exp/slog"
)

func Test(t *testing.T) {
	tr := &Tracer{}

	ctx := context.Background()
	ctx, s := tr.Start(ctx, "main")
	defer s.End()
	logger := slog.New(&handler{slog.Default().Handler()}).WithContext(ctx)
	logger.Info("in main")
	f(ctx, tr, logger)
	logger.Info("still in main")
}

func f(ctx context.Context, tr *Tracer, l *slog.Logger) {
	ctx, s := tr.Start(ctx, "f")
	defer s.End()
	l.Info("in f")
	g(ctx, tr, l)
	l.Info("still in f")
}

func g(ctx context.Context, tr *Tracer, l *slog.Logger) {
	ctx, s := tr.Start(ctx, "g")
	defer s.End()
	l.Info("in g")
}

type handler struct {
	slog.Handler
}

func (h *handler) Handle(r slog.Record) error {
	if name := SpanName(r.Context); name != "" {
		r.AddAttrs(slog.String("span", name))
	}
	return h.Handler.Handle(r)
}
