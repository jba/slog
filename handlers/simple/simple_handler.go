package simple

import (
	"context"
	"log/slog"
	"slices"

	"github.com/jba/slog/withsupport"
)

func Handler(handle func(slog.Record) error, opts slog.HandlerOptions) slog.Handler {
	return &simpleHandler{opts, handle, nil}
}

type simpleHandler struct {
	opts   slog.HandlerOptions
	handle func(slog.Record) error
	goa    *withsupport.GroupOrAttrs
}

func (h *simpleHandler) Enabled(ctx context.Context, l slog.Level) bool {
	minLevel := slog.LevelInfo
	if h.opts.Level != nil {
		minLevel = h.opts.Level.Level()
	}
	return l >= minLevel
}

func (h *simpleHandler) WithGroup(name string) slog.Handler {
	h2 := *h
	h2.goa = h2.goa.WithGroup(name)
	return &h2
}

func (h *simpleHandler) WithAttrs(as []slog.Attr) slog.Handler {
	h2 := *h
	h2.goa = h2.goa.WithAttrs(as)
	return &h2
}

func (h *simpleHandler) Handle(ctx context.Context, r slog.Record) error {
	r2 := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)

	var attrs []slog.Attr
	r.Attrs(func(a slog.Attr) bool { attrs = append(attrs, a); return true })
	for g := h.goa; g != nil; g = g.Next {
		if g.Group != "" {
			anys := make([]any, len(attrs))
			for i, a := range attrs {
				anys[i] = a
			}
			attrs = []slog.Attr{slog.Group(g.Group, anys...)}
		} else {
			attrs = append(slices.Clip(g.Attrs), attrs...)
		}
	}
	r2.AddAttrs(attrs...)
	return h.handle(r2)
}
