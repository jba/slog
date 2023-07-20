package gokit

import (
	"bytes"
	"strings"
	"testing"

	gklevel "github.com/go-kit/log/level"
	"golang.org/x/exp/slog"
)

func Test(t *testing.T) {
	var buf bytes.Buffer
	h := slog.NewTextHandler(&buf, nil)
	logger := New(h, "message")
	logger = gklevel.NewInjector(logger, gklevel.WarnValue())
	if err := logger.Log("message", "hello", "a", 1, "b", true); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(buf.String())
	want := `level=WARN msg=hello a=1 b=true`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
