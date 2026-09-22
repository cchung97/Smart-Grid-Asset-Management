package logs

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

// withCapturedOutput points the package handler at buf for the duration
// of the test and restores the previous output/level on cleanup — the
// handler is a package-level singleton shared across tests.
func withCapturedOutput(t *testing.T) *bytes.Buffer {
	t.Helper()
	prevLevel := level.Level()
	prevHandler := handler.Load()
	buf := &bytes.Buffer{}
	SetOutput(buf)
	t.Cleanup(func() {
		SetLevel(prevLevel)
		handler.Store(prevHandler)
	})
	return buf
}

func TestInfo_TraceIDField(t *testing.T) {
	tests := []struct {
		name       string
		ctx        context.Context
		wantField  string
		wantAbsent bool
	}{
		{
			name:      "context with trace ID includes tid field",
			ctx:       ContextWithTraceID(context.Background(), "abc123"),
			wantField: "tid=abc123",
		},
		{
			name:       "context without trace ID omits tid field",
			ctx:        context.Background(),
			wantAbsent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := withCapturedOutput(t)

			WithCtx(tt.ctx).Info("hello")

			out := buf.String()
			if !strings.Contains(out, `msg=hello`) {
				t.Fatalf("expected output to contain msg=hello, got: %s", out)
			}
			if tt.wantAbsent {
				if strings.Contains(out, "tid=") {
					t.Fatalf("expected no tid field, got: %s", out)
				}
				return
			}
			if !strings.Contains(out, tt.wantField) {
				t.Fatalf("expected output to contain %q, got: %s", tt.wantField, out)
			}
		})
	}
}

func TestInfo_FieldOrderAndSource(t *testing.T) {
	buf := withCapturedOutput(t)

	WithCtx(ContextWithTraceID(context.Background(), "abc123")).Warn("rate limit exceeded", "ip", "127.0.0.1")

	out := buf.String()
	tsIdx := strings.Index(out, "ts=")
	tidIdx := strings.Index(out, "tid=")
	lvlIdx := strings.Index(out, "lvl=")
	srcIdx := strings.Index(out, "src=")
	msgIdx := strings.Index(out, "msg=")
	if tsIdx < 0 || tidIdx < 0 || lvlIdx < 0 || srcIdx < 0 || msgIdx < 0 {
		t.Fatalf("expected ts/tid/lvl/src/msg fields, got: %s", out)
	}
	if !(tsIdx < tidIdx && tidIdx < lvlIdx && lvlIdx < srcIdx && srcIdx < msgIdx) {
		t.Fatalf("expected field order ts, tid, lvl, src, msg, got: %s", out)
	}
	// src must point at the caller of Warn (this test file), not a frame
	// inside the logs package itself.
	if !strings.Contains(out, "src=logs_test.go:") {
		t.Fatalf("expected src to point at the caller, got: %s", out)
	}
}

func TestSetLevel_Filtering(t *testing.T) {
	buf := withCapturedOutput(t)
	SetLevel(slog.LevelWarn)
	l := WithCtx(context.Background())

	l.Debug("debug message")
	l.Info("info message")
	if buf.Len() != 0 {
		t.Fatalf("expected no output below the Warn threshold, got: %s", buf.String())
	}

	l.Warn("warn message")
	if !strings.Contains(buf.String(), "warn message") {
		t.Fatalf("expected Warn output at the Warn threshold, got: %s", buf.String())
	}

	buf.Reset()
	l.Error("error message")
	if !strings.Contains(buf.String(), "error message") {
		t.Fatalf("expected Error output at the Warn threshold, got: %s", buf.String())
	}
}

// A user-controlled value (upload filename, CSV cell) must not be able to
// end its log line and start a forged one.
func TestInfo_ControlCharactersCannotForgeALogLine(t *testing.T) {
	buf := withCapturedOutput(t)
	WithCtx(context.Background()).Info("import row rejected",
		"asset_id", "A1\nts=2000-01-01 lvl=ERROR msg=forged", "file", "ok.csv", "tab", "a\tb", "plain", "value")

	out := buf.String()
	if got := strings.Count(out, "\n"); got != 1 {
		t.Fatalf("output has %d newlines, want exactly the line terminator:\n%q", got, out)
	}
	for _, want := range []string{`asset_id="A1\nts=2000-01-01 lvl=ERROR msg=forged"`, `tab="a\tb"`, "file=ok.csv", "plain=value"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %s:\n%s", want, out)
		}
	}
}
