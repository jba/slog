package handlers

import (
	"io"

	"golang.org/x/exp/slog"
)

// BinaryHandler uses the format in github.com/jba/slog/binary
type BinaryHandler struct {
	w     io.Writer
	level slog.Leveler
}

func NewBinaryHandler(w io.Writer, level slog.Leveler) *BinaryHandler {
	if level == nil {
		level = slog.LevelInfo
	}
	return &BinaryHandler{
		w:     w,
		level: level,
	}
}

func (h *BinaryHandler) Enabled(l slog.Level) bool {
	return l >= h.level.Level()
}
