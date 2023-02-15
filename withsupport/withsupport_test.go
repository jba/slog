package withsupport

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"golang.org/x/exp/slog"
)

type handler struct {
	w    io.Writer
	with *GroupOrAttrs
}

func (h *handler) Enabled(ctx context.Context, l slog.Level) bool {
	return true
}

func (h *handler) WithGroup(name string) slog.Handler {
	return &handler{h.w, h.with.WithGroup(name)}
}

func (h *handler) WithAttrs(as []slog.Attr) slog.Handler {
	return &handler{h.w, h.with.WithAttrs(as)}
}

func (h *handler) Handle(r slog.Record) error {
	fmt.Fprintf(h.w, "level=%s", r.Level)
	fmt.Fprintf(h.w, " msg=%q", r.Message)

	groups := h.with.Apply(h.formatAttr)
	r.Attrs(func(a slog.Attr) {
		h.formatAttr(groups, a)
	})
	return nil
}

func (h *handler) formatAttr(groups []string, a slog.Attr) {
	if a.Value.Kind() == slog.KindGroup {
		gs := a.Value.Group()
		if len(gs) == 0 {
			return
		}
		if a.Key != "" {
			groups = append(groups, a.Key)
		}
		for _, g := range gs {
			h.formatAttr(groups, g)
		}
	} else if key := a.Key; key != "" {
		if len(groups) > 0 {
			key = strings.Join(groups, ".") + "." + key
		}
		fmt.Fprintf(h.w, " %s=%s", key, a.Value)
	}
}

func Test(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(&handler{&buf, nil})
	logger.With("a", 1).
		WithGroup("G").
		With("b", 2).
		WithGroup("H").
		Info("msg", "c", 3, slog.Group("I", slog.Int("d", 4)), "e", 5)
	got := buf.String()
	want := `level=INFO msg="msg" a=1 G.b=2 G.H.c=3 G.H.I.d=4 G.H.e=5`
	if got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}
