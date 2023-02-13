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
	key := a.Key
	if len(groups) > 0 {
		key = strings.Join(groups, ".") + "." + key
	}
	fmt.Fprintf(h.w, " %s=%s", key, a.Value)
}

func Test(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(&handler{&buf, nil})
	logger.With("a", 1).
		WithGroup("G").
		With("b", 2).
		WithGroup("H").
		Info("msg", "c", 3)
	got := buf.String()
	want := `level=INFO msg="msg" a=1 G.b=2 G.H.c=3`
	if got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}
