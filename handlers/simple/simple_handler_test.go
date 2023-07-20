package simple

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"golang.org/x/exp/slog"
)

func newHandle(w io.Writer) func(slog.Record) error {
	return func(r slog.Record) (err error) {
		fmt.Fprintf(w, "level=%s", r.Level)
		fmt.Fprintf(w, " msg=%q", r.Message)
		r.Attrs(func(a slog.Attr) bool {
			if err == nil {
				err = formatAttr(w, a)
			}
			return true
		})
		return err
	}
}

func formatAttr(w io.Writer, a slog.Attr) error {
	if a.Value.Kind() == slog.KindGroup {
		gs := a.Value.Group()
		if len(gs) == 0 {
			return nil
		}
		if a.Key != "" {
			fmt.Fprintf(w, " (%s)", a.Key)
		}
		for _, g := range gs {
			if err := formatAttr(w, g); err != nil {
				return err
			}
		}
		return nil
	}
	if a.Key == "" {
		return nil
	}
	_, err := fmt.Fprintf(w, " %s=%s", a.Key, a.Value)
	return err
}

func Test(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(Handler(newHandle(&buf), slog.HandlerOptions{}))
	logger.With("a", 1).
		WithGroup("G").
		With("b", 2).
		WithGroup("H").
		Info("msg", "c", 3)
	got := buf.String()
	want := `level=INFO msg="msg" a=1 (G) b=2 (H) c=3`
	if got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}
