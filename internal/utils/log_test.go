package utils

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

func TestLogAndDebug(t *testing.T) {
	var buf bytes.Buffer
	SetLogger(log.New(&buf, "", 0))
	SetDebug(true)
	t.Cleanup(func() {
		SetLogger(nil)
		SetDebug(false)
	})

	Log("hello")
	Debug("debug")
	out := buf.String()
	if !strings.Contains(out, "hello") || !strings.Contains(out, "debug") {
		t.Fatalf("unexpected log output: %q", out)
	}
}

func TestLog_NoLogger(_ *testing.T) {
	SetLogger(nil)
	SetDebug(false)
	Log("nope")
	Debug("nope")
}
