// Package gokit provides a go-kit/log.Logger that uses a slog.Handler.
//
// This is a PROOF OF CONCEPT. It is not production-ready.
package gokit

import (
	"context"
	"fmt"
	"time"

	gklog "github.com/go-kit/log"
	gklevel "github.com/go-kit/log/level"
	"golang.org/x/exp/slog"
)

// New returns go-kit logger that calls h.Handle.
//
// If messageKey is not empty, it is used to extract the
// message from the list of key-value pairs.
//
// The logger looks for a level (a value of type go-kit/log/level.Value)
// in the key-value pairs, extracts it, and converts it to a slog.Level.
//
// The logger does nothing with timestamps. It calls the handler with a zero
// timestamp, which should suppress the default timestamp output. If there is a
// timestamp in the key-value pairs, it will be output as an ordinary attribute.
func New(h slog.Handler, messageKey string) gklog.Logger {
	return &logger{h, messageKey}
}

type logger struct {
	h          slog.Handler
	messageKey string
}

func (l *logger) Log(keyvals ...any) error {
	var attrs []slog.Attr
	var (
		message string
		gkl     gklevel.Value
	)
	for i := 1; i < len(keyvals); i += 2 {
		key, ok := keyvals[i-1].(string)
		// go-kit/log keys don't have to be strings, but slog keys do.
		// Convert the go-kit key to a string with fmt.Sprint.
		if !ok {
			key = fmt.Sprint(keyvals[i-1])
		}
		if l.messageKey != "" && key == l.messageKey {
			message = fmt.Sprint(keyvals[i])
			continue
		}
		if l, ok := keyvals[i].(gklevel.Value); ok {
			gkl = l
			continue
		}
		attrs = append(attrs, slog.Any(key, keyvals[i]))
	}
	var sl slog.Level
	if gkl != nil {
		switch gkl {
		case gklevel.DebugValue():
			sl = slog.LevelDebug
		case gklevel.InfoValue():
			sl = slog.LevelInfo
		case gklevel.WarnValue():
			sl = slog.LevelWarn
		case gklevel.ErrorValue():
			sl = slog.LevelError
		}
	}
	r := slog.NewRecord(time.Time{}, sl, message, 0)
	r.AddAttrs(attrs...)
	return l.h.Handle(context.Background(), r)
}
