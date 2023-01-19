package general

import (
	"fmt"
	"io"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/exp/slices"
	"golang.org/x/exp/slog"
)

type Handler struct {
	opts         Options
	newFormatter func() Formatter
	preformatted []byte
	groups       []string
	mu           sync.Mutex
	w            io.Writer
}

type Options struct {
	Level       slog.Leveler
	ReplaceAttr func(groups []string, a slog.Attr) slog.Attr
	FrameAttrs  func(runtime.Frame) []slog.Attr
}

func New(w io.Writer, newFormatter func() Formatter) *Handler {
	return Options{}.New(w, newFormatter)
}

func (opts Options) New(w io.Writer, newFormatter func() Formatter) *Handler {
	return &Handler{
		w:            w,
		opts:         opts,
		newFormatter: newFormatter,
	}
}

func (h *Handler) Enabled(level slog.Level) bool {
	return level >= h.opts.Level.Level()
}

func (h *Handler) Handle(r slog.Record) error {
	buf := make([]byte, 0, 1024) // TODO: use a sync.Pool.
	f := h.newFormatter()
	buf = f.AppendBegin(buf)
	if !r.Time.IsZero() {
		buf = h.appendAttr(buf, f, slog.Time(slog.TimeKey, r.Time))
	}
	buf = h.appendAttr(buf, f, slog.Any(slog.LevelKey, r.Level))
	buf = h.appendAttr(buf, f, slog.String(slog.MessageKey, r.Message))
	if h.opts.FrameAttrs != nil {
		for _, a := range h.opts.FrameAttrs(r.Frame()) {
			buf = h.appendAttr(buf, f, a)
		}
	}
	buf = f.AppendPreformatted(buf, h.preformatted, h.groups)
	r.Attrs(func(a slog.Attr) {
		buf = h.appendAttr(buf, f, a)
	})
	buf = f.AppendEnd(buf)
	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.w.Write(buf)
	return err
}

func (h *Handler) WithGroup(name string) slog.Handler {
	c := h.clone()
	c.groups = append(c.groups, name)
	c.preformatted = c.pref.OpenGroup(c.preformatted, name)
	return c
}

func (h *Handler) WithAttrs(as []slog.Attr) slog.Handler {
	c := h.clone()

	f.GroupsOpen()
	for _, a := range as {
		c.formatAttr(f, a)
	}
	c.preformatted = f.Bytes()
	return c
}

func (h *Handler) appendAttr(buf []byte, f Formatter, a slog.Attr) []byte {
	if h.opts.ReplaceAttr != nil {
		a = h.opts.ReplaceAttr(h.groups, a)
	}
	if a.Key != "" {
		return f.AppendAttr(buf, a)
	}
	return buf
}

func (h *Handler) clone() *Handler {
	c := *h
	c.groups = slices.Clip(c.groups)
	c.preformatted = slices.Clip(c.preformatted)
	return &c
}

////////////////////////////////////////////////////////////////

type Formatter interface {
	AppendBegin([]byte) []byte
	AppendEnd([]byte) []byte
	AppendAttr([]byte, slog.Attr) []byte
	AppendPreformatted(dst, pre []byte, openGroups []string) []byte
}

////////////////////////////////////////////////////////////////

type indentingFormatter struct {
	buf        []byte
	indent     int
	openGroups []string
}

// func NewIndentingFormatter(buf []byte) Formatter {
// 	return &indentingFormatter{buf: buf, indent: 0}
// }

func (f *indentingFormatter) OpenGroup(name string) {
	f.appendIndent()
	f.buf = append(f.buf, name...)
	f.buf = append(f.buf, ":\n"...)
	f.indent++
}

func (f *indentingFormatter) CloseGroup() {
	f.indent--
}

func (f *indentingFormatter) GroupsOpen(names []string) {
	f.openGroups = names
}

func (f *indentingFormatter) CloseOpenGroups() {
	for i := len(f.openGroups) - 1; i >= 0; i-- {
		f.indent--
		f.appendIndent()
		f.buf = append(f.buf, "end "...)
		f.buf = append(f.buf, f.openGroups[i]...)
		f.buf = append(f.buf, '\n')
	}
	f.openGroups = nil
}

func (f *indentingFormatter) appendIndent() {
	f.buf = append(f.buf, strings.Repeat("    ", f.indent)...)
}

func (f *indentingFormatter) Attr(a slog.Attr) {
	if a.Value.Kind() == slog.KindGroup {
		if a.Key != "" {
			f.OpenGroup(a.Key)
		}
		for _, a2 := range a.Value.Group() {
			f.Attr(a2)
		}
		if a.Key != "" {
			f.CloseGroup()
		}
	} else {
		f.appendIndent()
		f.buf = fmt.Appendf(f.buf, "%s: %s\n", a.Key, a.Value)
	}
}

func (f *indentingFormatter) Bytes() []byte {
	return f.buf
}

////////////////////////////////////////////////////////////////

type jsonFormatter struct {
	nOpenGroups int
	needComma   bool
}

func newJSONFormatter() Formatter {
	return &jsonFormatter{nOpenGroups: 0}
}

func (f *jsonFormatter) AppendBegin(buf []byte) []byte {
	return append(buf, '{')
}
func (f *jsonFormatter) AppendEnd(buf []byte) []byte {
	for i := 0; i < nOpenGroups+1; i++ {
		buf = append(buf, '}')
	}
	return buf
}

func (f *jsonFormatter) AppendAttr(buf []byte, a slog.Attr) []byte {
	if a.Value.Kind() == slog.KindGroup {
		if a.Key != "" {
			buf = fmt.Appendf(buf, "{%q:", a.Key)
		}
		for _, a2 := range a.Value.Group() {
			ubf = f.AppendAttr(a2)
		}
		if a.Key != "" {
			buf = append(buf, '}')
		}
	} else {
		if f.needComma {
			buf = append(buf, ',')
			f.needComma = false
		}
		buf = fmt.Appendf(buf, "%q:", a.Key)
		v := a.Value
		switch v.Kind() {
		case slog.KindInt64:
			buf = strconv.AppendInt(buf, v.Int64(), 10)
		case slog.KindTime:
			buf = strconv.AppendQuote(buf, v.Time().Format(time.RFC3339))
		default:
			buf = strconv.AppendQuote(buf, v.String())
		}
		f.needComma = true
	}
	return buf
}
