// Package verbosity converts between logr-style verbosities
// and slog Levels.
//
// These functions implement the equation
//
//	Level = INFO - verbosity
package verbosity

import "log/slog"

// ToLevel converts a verbosity to a Level.
func ToLevel(verbosity int) slog.Level {
	return slog.LevelInfo - slog.Level(verbosity)
}

// FromLevel converts a Level to a verbosity.
func FromLevel(l slog.Level) int {
	return int(slog.LevelInfo - l)
}
