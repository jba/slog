package general

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"golang.org/x/exp/slog"
)

type Attr = slog.Attr

func TestHandler(t *testing.T) {
	// ReplaceAttr functions

	// remove all Attrs
	removeAll := func(_ []string, a Attr) Attr { return Attr{} }

	attrs := []Attr{slog.String("a", "one"), slog.Int("b", 2), slog.Any("", "ignore me")}
	preAttrs := []Attr{slog.Int("pre", 3), slog.String("x", "y")}

	for _, test := range []struct {
		name     string
		replace  func([]string, Attr) Attr
		with     func(slog.Handler) slog.Handler
		preAttrs []Attr
		attrs    []Attr
		wantText string
		wantJSON string
	}{
		{
			name:     "basic",
			attrs:    attrs,
			wantText: "time=2000-01-02T03:04:05.000Z level=INFO msg=message a=one b=2",
			wantJSON: `{"time":"2000-01-02T03:04:05Z","level":"INFO","msg":"message","a":"one","b":2}`,
		},
		{
			name:     "cap keys",
			replace:  upperCaseKey,
			attrs:    attrs,
			wantText: "TIME=2000-01-02T03:04:05.000Z LEVEL=INFO MSG=message A=one B=2",
			wantJSON: `{"TIME":"2000-01-02T03:04:05Z","LEVEL":"INFO","MSG":"message","A":"one","B":2}`,
		},
		{
			name:     "remove all",
			replace:  removeAll,
			attrs:    attrs,
			wantText: "",
			wantJSON: `{}`,
		},
		{
			name:     "preformatted",
			with:     func(h slog.Handler) slog.Handler { return h.WithAttrs(preAttrs) },
			preAttrs: preAttrs,
			attrs:    attrs,
			wantText: "time=2000-01-02T03:04:05.000Z level=INFO msg=message pre=3 x=y a=one b=2",
			wantJSON: `{"time":"2000-01-02T03:04:05Z","level":"INFO","msg":"message","pre":3,"x":"y","a":"one","b":2}`,
		},
		{
			name:     "preformatted cap keys",
			replace:  upperCaseKey,
			with:     func(h slog.Handler) slog.Handler { return h.WithAttrs(preAttrs) },
			preAttrs: preAttrs,
			attrs:    attrs,
			wantText: "TIME=2000-01-02T03:04:05.000Z LEVEL=INFO MSG=message PRE=3 X=y A=one B=2",
			wantJSON: `{"TIME":"2000-01-02T03:04:05Z","LEVEL":"INFO","MSG":"message","PRE":3,"X":"y","A":"one","B":2}`,
		},
		{
			name:     "preformatted remove all",
			replace:  removeAll,
			with:     func(h slog.Handler) slog.Handler { return h.WithAttrs(preAttrs) },
			preAttrs: preAttrs,
			attrs:    attrs,
			wantText: "",
			wantJSON: "{}",
		},
		{
			name:     "remove built-in",
			replace:  removeKeys(slog.TimeKey, slog.LevelKey, slog.MessageKey),
			attrs:    attrs,
			wantText: "a=one b=2",
			wantJSON: `{"a":"one","b":2}`,
		},
		{
			name:     "preformatted remove built-in",
			replace:  removeKeys(slog.TimeKey, slog.LevelKey, slog.MessageKey),
			with:     func(h slog.Handler) slog.Handler { return h.WithAttrs(preAttrs) },
			attrs:    attrs,
			wantText: "pre=3 x=y a=one b=2",
			wantJSON: `{"pre":3,"x":"y","a":"one","b":2}`,
		},
		{
			name:    "groups",
			replace: removeKeys(slog.TimeKey, slog.LevelKey), // to simplify the result
			attrs: []Attr{
				slog.Int("a", 1),
				slog.Group("g",
					slog.Int("b", 2),
					slog.Group("h", slog.Int("c", 3)),
					slog.Int("d", 4)),
				slog.Int("e", 5),
			},
			wantText: "msg=message a=1 g.b=2 g.h.c=3 g.d=4 e=5",
			wantJSON: `{"msg":"message","a":1,"g":{"b":2,"h":{"c":3},"d":4},"e":5}`,
		},
		{
			name:     "empty group",
			replace:  removeKeys(slog.TimeKey, slog.LevelKey),
			attrs:    []Attr{slog.Group("g"), slog.Group("h", slog.Int("a", 1))},
			wantText: "msg=message h.a=1",
			wantJSON: `{"msg":"message","g":{},"h":{"a":1}}`,
		},
		{
			name:    "escapes",
			replace: removeKeys(slog.TimeKey, slog.LevelKey),
			attrs: []Attr{
				slog.String("a b", "x\t\n\000y"),
				slog.Group(" b.c=\"\\x2E\t",
					slog.String("d=e", "f.g\""),
					slog.Int("m.d", 1)), // dot is not escaped
			},
			wantText: `msg=message "a b"="x\t\n\x00y" " b.c=\"\\x2E\t.d=e"="f.g\"" " b.c=\"\\x2E\t.m.d"=1`,
			wantJSON: `{"msg":"message","a b":"x\t\n\u0000y"," b.c=\"\\x2E\t":{"d=e":"f.g\"","m.d":1}}`,
		},
		{
			name:    "LogValuer",
			replace: removeKeys(slog.TimeKey, slog.LevelKey),
			attrs: []Attr{
				slog.Int("a", 1),
				slog.Any("name", logValueName{"Ren", "Hoek"}),
				slog.Int("b", 2),
			},
			wantText: "msg=message a=1 name.first=Ren name.last=Hoek b=2",
			wantJSON: `{"msg":"message","a":1,"name":{"first":"Ren","last":"Hoek"},"b":2}`,
		},
		{
			name:     "with-group",
			replace:  removeKeys(slog.TimeKey, slog.LevelKey),
			with:     func(h slog.Handler) slog.Handler { return h.WithAttrs(preAttrs).WithGroup("s") },
			attrs:    attrs,
			wantText: "msg=message pre=3 x=y s.a=one s.b=2",
			wantJSON: `{"msg":"message","pre":3,"x":"y","s":{"a":"one","b":2}}`,
		},
		{
			name:    "preformatted with-groups",
			replace: removeKeys(slog.TimeKey, slog.LevelKey),
			with: func(h slog.Handler) slog.Handler {
				return h.WithAttrs([]Attr{slog.Int("p1", 1)}).
					WithGroup("s1").
					WithAttrs([]Attr{slog.Int("p2", 2)}).
					WithGroup("s2")
			},
			attrs:    attrs,
			wantText: "msg=message p1=1 s1.p2=2 s1.s2.a=one s1.s2.b=2",
			wantJSON: `{"msg":"message","p1":1,"s1":{"p2":2,"s2":{"a":"one","b":2}}}`,
		},
		{
			name:    "two with-groups",
			replace: removeKeys(slog.TimeKey, slog.LevelKey),
			with: func(h slog.Handler) slog.Handler {
				return h.WithAttrs([]Attr{slog.Int("p1", 1)}).
					WithGroup("s1").
					WithGroup("s2")
			},
			attrs:    attrs,
			wantText: "msg=message p1=1 s1.s2.a=one s1.s2.b=2",
			wantJSON: `{"msg":"message","p1":1,"s1":{"s2":{"a":"one","b":2}}}`,
		},
		{
			name:     "GroupValue as Attr value",
			replace:  removeKeys(slog.TimeKey, slog.LevelKey),
			attrs:    []Attr{{"v", slog.AnyValue(slog.IntValue(3))}},
			wantText: "msg=message v=3",
			wantJSON: `{"msg":"message","v":3}`,
		},
		{
			name:     "byte slice",
			replace:  removeKeys(slog.TimeKey, slog.LevelKey),
			attrs:    []Attr{slog.Any("bs", []byte{1, 2, 3, 4})},
			wantText: `msg=message bs="\x01\x02\x03\x04"`,
			wantJSON: `{"msg":"message","bs":"AQIDBA=="}`,
		},
		{
			name:     "json.RawMessage",
			replace:  removeKeys(slog.TimeKey, slog.LevelKey),
			attrs:    []Attr{slog.Any("bs", json.RawMessage([]byte("1234")))},
			wantText: `msg=message bs="1234"`,
			wantJSON: `{"msg":"message","bs":1234}`,
		},
	} {
		r := slog.NewRecord(testTime, slog.LevelInfo, "message", 1, nil)
		r.AddAttrs(test.attrs...)
		var buf bytes.Buffer
		opts := Options{ReplaceAttr: test.replace}
		t.Run(test.name, func(t *testing.T) {
			for _, handler := range []struct {
				name string
				h    slog.Handler
				want string
			}{
				{"json", opts.New(&buf, func(bs []byte) Formatter { return newJSONFormatter(bs) }), test.wantJSON},
			} {
				t.Run(handler.name, func(t *testing.T) {
					h := handler.h
					if test.with != nil {
						h = test.with(h)
					}
					buf.Reset()
					if err := h.Handle(r); err != nil {
						t.Fatal(err)
					}
					got := strings.TrimSuffix(buf.String(), "\n")
					if got != handler.want {
						t.Errorf("\ngot  %s\nwant %s\n", got, handler.want)
					}
				})
			}
		})
	}
}

// removeKeys returns a function suitable for HandlerOptions.ReplaceAttr
// that removes all Attrs with the given keys.
func removeKeys(keys ...string) func([]string, Attr) Attr {
	return func(_ []string, a Attr) Attr {
		for _, k := range keys {
			if a.Key == k {
				return Attr{}
			}
		}
		return a
	}
}

func upperCaseKey(_ []string, a Attr) Attr {
	a.Key = strings.ToUpper(a.Key)
	return a
}

type logValueName struct {
	first, last string
}

func (n logValueName) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("first", n.first),
		slog.String("last", n.last))
}

var testTime = time.Date(2000, 1, 2, 3, 4, 5, 0, time.UTC)
