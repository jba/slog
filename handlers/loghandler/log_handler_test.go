package loghandler

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"golang.org/x/exp/slog"
)

var testTime = time.Date(2023, time.April, 3, 1, 2, 3, 0, time.UTC)

func TestOutput(t *testing.T) {
	for _, test := range []struct {
		name    string
		handler func(io.Writer, *slog.HandlerOptions) *Handler
		with    func(*slog.Logger) *slog.Logger
		attrs   []slog.Attr
		want    string
	}{
		{
			name:    "basic",
			handler: New,
			attrs:   []slog.Attr{slog.String("c", "foo"), slog.Bool("b", true)},
			want:    `2023-04-03T01:02:03Z INFO message c=foo b=true`,
		},
		{
			name:    "group",
			handler: New,
			attrs: []slog.Attr{
				slog.String("c", "foo"),
				slog.Group("g", slog.Int("a", 1), slog.Int("d", 4)),
				slog.Bool("b", true),
			},
			want: `2023-04-03T01:02:03Z INFO message c=foo g.a=1 g.d=4 b=true`,
		},
		{
			name:    "WithAttrs",
			handler: New,
			with:    func(l *slog.Logger) *slog.Logger { return l.With("wa", 1, "wb", 2) },
			attrs:   []slog.Attr{slog.String("c", "foo"), slog.Bool("b", true)},
			want:    `2023-04-03T01:02:03Z INFO message wa=1 wb=2 c=foo b=true`,
		},
		{
			name:    "WithAttrs,WithGroup",
			handler: New,
			with: func(l *slog.Logger) *slog.Logger {
				return l.With("wa", 1, "wb", 2).WithGroup("p1").With("wc", 3).WithGroup("p2")
			},
			attrs: []slog.Attr{slog.String("c", "foo"), slog.Bool("b", true)},
			want:  `2023-04-03T01:02:03Z INFO message wa=1 wb=2 p1.wc=3 p1.p2.c=foo p1.p2.b=true`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var buf bytes.Buffer
			h := test.handler(&buf, nil)
			logger := slog.New(setTimeHandler{testTime, h})
			if test.with != nil {
				logger = test.with(logger)
			}
			logger.LogAttrs(context.Background(), slog.LevelInfo, "message", test.attrs...)
			got := buf.String()
			// remove final newline
			got = got[:len(got)-1]
			if got != test.want {
				t.Errorf("\ngot  %s\nwant %s", got, test.want)
			}
		})
	}
}

type setTimeHandler struct {
	t time.Time
	h slog.Handler
}

func (h setTimeHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.h.Enabled(ctx, level)
}

func (h setTimeHandler) WithGroup(name string) slog.Handler {
	return setTimeHandler{h.t, h.h.WithGroup(name)}
}

func (h setTimeHandler) WithAttrs(as []slog.Attr) slog.Handler {
	return setTimeHandler{h.t, h.h.WithAttrs(as)}
}

func (h setTimeHandler) Handle(ctx context.Context, r slog.Record) error {
	r.Time = h.t
	return h.h.Handle(ctx, r)
}
