package verbosity

import (
	"testing"

	"log/slog"
)

func TestToLevel(t *testing.T) {
	for _, test := range []struct {
		in   int
		want slog.Level
	}{
		{0, slog.LevelInfo},
		{1, slog.LevelDebug + 3},
		{4, slog.LevelDebug},
		{-4, slog.LevelWarn},
		{-8, slog.LevelError},
	} {
		got := ToLevel(test.in)
		if got != test.want {
			t.Errorf("%d: got %s, want %s", test.in, got, test.want)
		}
	}
}

func TestFromLevel(t *testing.T) {
	for _, test := range []struct {
		in   slog.Level
		want int
	}{
		{slog.LevelInfo, 0},
		{slog.LevelDebug + 3, 1},
		{slog.LevelDebug, 4},
		{slog.LevelWarn, -4},
		{slog.LevelError, -8},
	} {
		got := FromLevel(test.in)
		if got != test.want {
			t.Errorf("%s: got %d, want %d", test.in, got, test.want)
		}
	}
}
