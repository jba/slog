package general

import (
	"context"
	"encoding"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/exp/slices"
	"golang.org/x/exp/slog"
)

// Handler implements a [slog.Handler] that can produce a variety of output
// formats. It uses the [Formatter] interface to produce output.
//
// The main requirement of a format is that preformatted material--groups
// and attributes that were added using WithGroup and WithAttrs--can be
// concatenated between the built-in attributes and the remaining attributes.
type Handler struct {
	opts         Options
	newFormatter func() Formatter
	preformatted []byte
	groups       []string
	mu           sync.Mutex
	w            io.Writer
}

// Options are options for a [Handler].
type Options struct {
	// Level reports the minimum level to log.
	// Levels with lower levels are discarded.
	// If nil, the Handler uses [slog.LevelInfo].
	Level slog.Leveler

	// ReplaceAttr rewrites Attrs.
	ReplaceAttr func(groups []string, a slog.Attr) slog.Attr

	// PCAttrs returns the Attrs to use for source location.
	// If nil, no source information is output.
	PCAttrs func(pc uintptr) []slog.Attr
}

// New constructs a Handler with the default options.
func New(w io.Writer, newFormatter func() Formatter) *Handler {
	return Options{}.New(w, newFormatter)
}

// New constructs a Handler with the given options.
func (opts Options) New(w io.Writer, newFormatter func() Formatter) *Handler {
	return &Handler{
		w:            w,
		opts:         opts,
		newFormatter: newFormatter,
	}
}

func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	minLevel := slog.LevelInfo
	if h.opts.Level != nil {
		minLevel = h.opts.Level.Level()
	}
	return level >= minLevel
}

func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	buf := make([]byte, 0, 1024) // TODO: use a sync.Pool.
	f := h.newFormatter()
	buf = f.AppendBegin(buf)
	if !r.Time.IsZero() {
		buf = h.appendAttr(buf, f, slog.Time(slog.TimeKey, r.Time), false)
	}
	buf = h.appendAttr(buf, f, slog.Any(slog.LevelKey, r.Level), false)
	buf = h.appendAttr(buf, f, slog.String(slog.MessageKey, r.Message), false)
	if h.opts.PCAttrs != nil {
		for _, a := range h.opts.PCAttrs(r.PC) {
			buf = h.appendAttr(buf, f, a, false)
		}
	}
	if len(h.preformatted) > 0 {
		buf = f.AppendSeparatorIfNeeded(buf)
		buf = append(buf, h.preformatted...)
	}
	r.Attrs(func(a slog.Attr) bool {
		buf = h.appendAttr(buf, f, a, true)
		return true
	})
	for i := len(h.groups) - 1; i >= 0; i-- {
		buf = f.AppendCloseGroup(buf, h.groups[i])
	}
	buf = f.AppendEnd(buf)
	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.w.Write(buf)
	return err
}

func (h *Handler) WithGroup(name string) slog.Handler {
	c := h.clone()
	c.groups = append(c.groups, name)
	f := c.newFormatter()
	c.preformatted = f.AppendOpenGroup(c.preformatted, name)
	return c
}

func (h *Handler) WithAttrs(as []slog.Attr) slog.Handler {
	c := h.clone()
	f := c.newFormatter()
	for _, a := range as {
		c.preformatted = c.appendAttr(c.preformatted, f, a, true)
	}
	return c
}

func (h *Handler) appendAttr(buf []byte, f Formatter, a slog.Attr, includeGroups bool) []byte {
	var groups []string
	if includeGroups {
		groups = h.groups
	}
	if h.opts.ReplaceAttr != nil {
		a = h.opts.ReplaceAttr(groups, a)
	}
	if a.Key != "" || a.Value.Kind() == slog.KindGroup {
		return f.AppendAttr(buf, a, groups)
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

// A Formatter formats log output.
// Each Formatter method takes a []byte buffer, which it should
// append to and then return.
type Formatter interface {
	// Append at the beginning of the log event.
	AppendBegin([]byte) []byte
	// Append at the end of the log event.
	AppendEnd([]byte) []byte
	// Append when a group with the given name starts.
	AppendOpenGroup(buf []byte, name string) []byte
	// Append when a group with the given name ends.
	AppendCloseGroup(buf []byte, name string) []byte
	// Append an Attr in the context of the given groups.
	AppendAttr(buf []byte, a slog.Attr, groups []string) []byte
	// Append a separator between Attrs, if one is needed.
	AppendSeparatorIfNeeded([]byte) []byte
}

////////////////////////////////////////////////////////////////

type jsonFormatter struct {
}

func newJSONFormatter() Formatter {
	return &jsonFormatter{}
}

func (f *jsonFormatter) AppendBegin(buf []byte) []byte {
	return append(buf, '{')
}

func (f *jsonFormatter) AppendEnd(buf []byte) []byte {
	return append(buf, '}')
}

func (f *jsonFormatter) AppendOpenGroup(buf []byte, name string) []byte {
	buf = f.AppendSeparatorIfNeeded(buf)
	return fmt.Appendf(buf, "%q:{", name)
}

func (f *jsonFormatter) AppendCloseGroup(buf []byte, name string) []byte {
	return append(buf, '}')
}

func (f *jsonFormatter) AppendSeparatorIfNeeded(buf []byte) []byte {
	if len(buf) > 0 && buf[len(buf)-1] != '{' && buf[len(buf)-1] != ',' {
		return append(buf, ',')
	}
	return buf
}

func (f *jsonFormatter) AppendAttr(buf []byte, a slog.Attr, openGroups []string) []byte {
	a.Value = a.Value.Resolve()
	buf = f.AppendSeparatorIfNeeded(buf)
	if a.Value.Kind() == slog.KindGroup {
		if a.Key != "" {
			buf = fmt.Appendf(buf, "%q:{", a.Key)
		}
		for _, a2 := range a.Value.Group() {
			buf = f.AppendAttr(buf, a2, openGroups)
		}
		if a.Key != "" {
			buf = append(buf, '}')
		}
	} else {
		buf = fmt.Appendf(buf, "%q:", a.Key)
		v := a.Value
		switch v.Kind() {
		case slog.KindString:
			buf = append(buf, '"')
			buf = appendEscapedJSONString(buf, v.String())
			buf = append(buf, '"')

		case slog.KindInt64:
			buf = strconv.AppendInt(buf, v.Int64(), 10)
		case slog.KindTime:
			buf = strconv.AppendQuote(buf, v.Time().Format(time.RFC3339))
		case slog.KindAny:
			a := v.Any()
			if err, ok := a.(error); ok {
				buf = append(buf, err.Error()...)
			} else {
				bs, err := json.Marshal(a)
				if err != nil {
					buf = append(buf, `"?"`...)
				} else {
					buf = append(buf, bs...)
				}
			}

		default:
			buf = appendEscapedJSONString(buf, v.String())
		}
	}
	return buf
}

////////////////////////////////////////////////////////////////

type indentingFormatter struct {
	indent int
}

func (f *indentingFormatter) appendIndent(buf []byte) []byte {
	return append(buf, strings.Repeat("  ", f.indent)...)
}

func (*indentingFormatter) AppendBegin(buf []byte) []byte { return buf }

func (*indentingFormatter) AppendEnd(buf []byte) []byte { return buf }

func (f *indentingFormatter) AppendOpenGroup(buf []byte, name string) []byte {
	buf = f.appendIndent(buf)
	f.indent++
	buf = append(buf, name...)
	return append(buf, ":\n"...)
}

func (f *indentingFormatter) AppendCloseGroup(buf []byte, name string) []byte {
	f.indent--
	return buf
}

func (indentingFormatter) AppendSeparatorIfNeeded(buf []byte) []byte { return buf }

func (f *indentingFormatter) AppendAttr(buf []byte, a slog.Attr, openGroups []string) []byte {
	if a.Value.Kind() == slog.KindGroup {
		if a.Key != "" {
			buf = f.AppendOpenGroup(buf, a.Key)
		}
		for _, a2 := range a.Value.Group() {
			buf = f.AppendAttr(buf, a2, openGroups)
		}
		if a.Key != "" {
			buf = f.AppendCloseGroup(buf, a.Key)
		}
		return buf
	} else {
		buf = f.appendIndent(buf)
		return fmt.Appendf(buf, "%s: %s\n", a.Key, a.Value)
	}
}

////////////////////////////////////////////////////////////////

func appendEscapedJSONString(buf []byte, s string) []byte {
	char := func(b byte) { buf = append(buf, b) }
	str := func(s string) { buf = append(buf, s...) }

	start := 0
	for i := 0; i < len(s); {
		if b := s[i]; b < utf8.RuneSelf {
			if htmlSafeSet[b] {
				i++
				continue
			}
			if start < i {
				str(s[start:i])
			}
			char('\\')
			switch b {
			case '\\', '"':
				char(b)
			case '\n':
				char('n')
			case '\r':
				char('r')
			case '\t':
				char('t')
			default:
				// This encodes bytes < 0x20 except for \t, \n and \r.
				// It also escapes <, >, and &
				// because they can lead to security holes when
				// user-controlled strings are rendered into JSON
				// and served to some browsers.
				str(`u00`)
				char(hex[b>>4])
				char(hex[b&0xF])
			}
			i++
			start = i
			continue
		}
		c, size := utf8.DecodeRuneInString(s[i:])
		if c == utf8.RuneError && size == 1 {
			if start < i {
				str(s[start:i])
			}
			str(`\ufffd`)
			i += size
			start = i
			continue
		}
		// U+2028 is LINE SEPARATOR.
		// U+2029 is PARAGRAPH SEPARATOR.
		// They are both technically valid characters in JSON strings,
		// but don't work in JSONP, which has to be evaluated as JavaScript,
		// and can lead to security holes there. It is valid JSON to
		// escape them, so we do so unconditionally.
		// See http://timelessrepo.com/json-isnt-a-javascript-subset for discussion.
		if c == '\u2028' || c == '\u2029' {
			if start < i {
				str(s[start:i])
			}
			str(`\u202`)
			char(hex[c&0xF])
			i += size
			start = i
			continue
		}
		i += size
	}
	if start < len(s) {
		str(s[start:])
	}
	return buf
}

var hex = "0123456789abcdef"

// Copied from encoding/json/encode.go:encodeState.string.
//
// htmlSafeSet holds the value true if the ASCII character with the given
// array position can be safely represented inside a JSON string, embedded
// inside of HTML <script> tags, without any additional escaping.
//
// All values are true except for the ASCII control characters (0-31), the
// double quote ("), the backslash character ("\"), HTML opening and closing
// tags ("<" and ">"), and the ampersand ("&").
var htmlSafeSet = [utf8.RuneSelf]bool{
	' ':      true,
	'!':      true,
	'"':      false,
	'#':      true,
	'$':      true,
	'%':      true,
	'&':      false,
	'\'':     true,
	'(':      true,
	')':      true,
	'*':      true,
	'+':      true,
	',':      true,
	'-':      true,
	'.':      true,
	'/':      true,
	'0':      true,
	'1':      true,
	'2':      true,
	'3':      true,
	'4':      true,
	'5':      true,
	'6':      true,
	'7':      true,
	'8':      true,
	'9':      true,
	':':      true,
	';':      true,
	'<':      false,
	'=':      true,
	'>':      false,
	'?':      true,
	'@':      true,
	'A':      true,
	'B':      true,
	'C':      true,
	'D':      true,
	'E':      true,
	'F':      true,
	'G':      true,
	'H':      true,
	'I':      true,
	'J':      true,
	'K':      true,
	'L':      true,
	'M':      true,
	'N':      true,
	'O':      true,
	'P':      true,
	'Q':      true,
	'R':      true,
	'S':      true,
	'T':      true,
	'U':      true,
	'V':      true,
	'W':      true,
	'X':      true,
	'Y':      true,
	'Z':      true,
	'[':      true,
	'\\':     false,
	']':      true,
	'^':      true,
	'_':      true,
	'`':      true,
	'a':      true,
	'b':      true,
	'c':      true,
	'd':      true,
	'e':      true,
	'f':      true,
	'g':      true,
	'h':      true,
	'i':      true,
	'j':      true,
	'k':      true,
	'l':      true,
	'm':      true,
	'n':      true,
	'o':      true,
	'p':      true,
	'q':      true,
	'r':      true,
	's':      true,
	't':      true,
	'u':      true,
	'v':      true,
	'w':      true,
	'x':      true,
	'y':      true,
	'z':      true,
	'{':      true,
	'|':      true,
	'}':      true,
	'~':      true,
	'\u007f': true,
}

////////////////////////////////////////////////////////////////

type textFormatter struct{}

func (textFormatter) AppendBegin(buf []byte) []byte {
	return buf
}

func (textFormatter) AppendEnd(buf []byte) []byte {
	return buf
}

func (textFormatter) AppendOpenGroup(buf []byte, name string) []byte {
	return buf
}

func (textFormatter) AppendCloseGroup(buf []byte, name string) []byte {
	return buf
}

func (textFormatter) AppendSeparatorIfNeeded(buf []byte) []byte {
	if len(buf) > 0 && buf[len(buf)-1] != ' ' {
		return append(buf, ' ')
	}
	return buf
}

func (f textFormatter) AppendAttr(buf []byte, a slog.Attr, openGroups []string) []byte {
	openGroups = slices.Clip(openGroups)
	a.Value = a.Value.Resolve()
	buf = f.AppendSeparatorIfNeeded(buf)
	if a.Value.Kind() == slog.KindGroup {
		if a.Key != "" {
			openGroups = append(openGroups, a.Key)
		}
		for _, a2 := range a.Value.Group() {
			buf = f.AppendAttr(buf, a2, openGroups)
		}
	} else {
		k := a.Key
		if len(openGroups) > 0 {
			k = strings.Join(openGroups, ".") + "." + k
		}
		buf = appendTextString(buf, k)
		buf = append(buf, '=')
		buf = appendTextValue(buf, a.Value)
	}
	return buf
}

func appendTextString(buf []byte, s string) []byte {
	if needsQuoting(s) {
		return strconv.AppendQuote(buf, s)
	} else {
		return append(buf, s...)
	}
}

func appendTextValue(buf []byte, v slog.Value) []byte {
	switch v.Kind() {
	case slog.KindString:
		return appendTextString(buf, v.String())
	case slog.KindTime:
		buf = appendTimeRFC3339Millis(buf, v.Time())
	case slog.KindAny:
		if tm, ok := v.Any().(encoding.TextMarshaler); ok {
			data, err := tm.MarshalText()
			if err != nil {
				buf = append(buf, err.Error()...)
			} else {
				buf = append(buf, string(data)...)
			}
			return buf
		}
		if bs, ok := byteSlice(v.Any()); ok {
			buf = append(buf, strconv.Quote(string(bs))...)
			return buf
		}
		buf = append(buf, fmt.Sprint(v.Any())...)
	default:
		buf = append(buf, fmt.Sprint(v.Any())...)
	}
	return buf
}

// byteSlice returns its argument as a []byte if the argument's
// underlying type is []byte, along with a second return value of true.
// Otherwise it returns nil, false.
func byteSlice(a any) ([]byte, bool) {
	if bs, ok := a.([]byte); ok {
		return bs, true
	}
	// Like Printf's %s, we allow both the slice type and the byte element type to be named.
	t := reflect.TypeOf(a)
	if t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8 {
		return reflect.ValueOf(a).Bytes(), true
	}
	return nil, false
}

func itoa(buf *[]byte, i int, wid int) {
	// Assemble decimal in reverse order.
	var b [20]byte
	bp := len(b) - 1
	for i >= 10 || wid > 1 {
		wid--
		q := i / 10
		b[bp] = byte('0' + i - q*10)
		bp--
		i = q
	}
	// i < 10
	b[bp] = byte('0' + i)
	*buf = append(*buf, b[bp:]...)
}

func appendTimeRFC3339Millis(buf []byte, t time.Time) []byte {
	year, month, day := t.Date()
	itoa(&buf, year, 4)
	buf = append(buf, '-')
	itoa(&buf, int(month), 2)
	buf = append(buf, '-')
	itoa(&buf, day, 2)
	buf = append(buf, 'T')
	hour, min, sec := t.Clock()
	itoa(&buf, hour, 2)
	buf = append(buf, ':')
	itoa(&buf, min, 2)
	buf = append(buf, ':')
	itoa(&buf, sec, 2)
	ns := t.Nanosecond()
	buf = append(buf, '.')
	itoa(&buf, ns/1e6, 3)
	_, offsetSeconds := t.Zone()
	if offsetSeconds == 0 {
		buf = append(buf, 'Z')
	} else {
		offsetMinutes := offsetSeconds / 60
		if offsetMinutes < 0 {
			buf = append(buf, '-')
			offsetMinutes = -offsetMinutes
		} else {
			buf = append(buf, '+')
		}
		itoa(&buf, offsetMinutes/60, 2)
		buf = append(buf, ':')
		itoa(&buf, offsetMinutes%60, 2)
	}
	return buf
}

func needsQuoting(s string) bool {
	for i := 0; i < len(s); {
		b := s[i]
		if b < utf8.RuneSelf {
			if needsQuotingSet[b] {
				return true
			}
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError || unicode.IsSpace(r) || !unicode.IsPrint(r) {
			return true
		}
		i += size
	}
	return false
}

var needsQuotingSet = [utf8.RuneSelf]bool{
	'"': true,
	'=': true,
}

func init() {
	for i := 0; i < utf8.RuneSelf; i++ {
		r := rune(i)
		if unicode.IsSpace(r) || !unicode.IsPrint(r) {
			needsQuotingSet[i] = true
		}
	}
}
