// Package logs is a compact structured logger built on log/slog's
// Record/Handler types: one line per call —
// ts=... tid=... lvl=... src=... msg=... key=val ... — with short field
// names and the trace ID (see ContextWithTraceID) pulled straight out
// of context. No adapters/sinks beyond one writer — add those back only
// if a real need shows up (see CLAUDE.md's non-negotiables).
package logs

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type traceIDKey struct{}

// ContextWithTraceID attaches a trace/request ID to ctx so it can be
// pulled back out by the logger (see lineHandler.Handle) or by handlers
// that want to echo it elsewhere (e.g. an error response body).
func ContextWithTraceID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, traceIDKey{}, id)
}

// TraceIDFromContext returns the trace ID stashed by ContextWithTraceID,
// or "" if ctx carries none.
func TraceIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(traceIDKey{}).(string)
	return id
}

// lineHandler renders one line per record in a fixed field order:
// ts, tid, lvl, src, msg, then caller-supplied key/value pairs. tid and
// src are left out when there's no trace ID or no caller PC.
type lineHandler struct {
	mu    sync.Mutex
	w     io.Writer
	level *slog.LevelVar
}

func (h *lineHandler) Enabled(_ context.Context, lvl slog.Level) bool {
	return lvl >= h.level.Level()
}

func (h *lineHandler) Handle(ctx context.Context, r slog.Record) error {
	var b strings.Builder

	b.WriteString("ts=")
	b.WriteString(r.Time.Format("2006-01-02 15:04:05.000"))

	if id := TraceIDFromContext(ctx); id != "" {
		b.WriteString(" tid=")
		b.WriteString(id)
	}

	b.WriteString(" lvl=")
	b.WriteString(r.Level.String())

	if r.PC != 0 {
		frame, _ := runtime.CallersFrames([]uintptr{r.PC}).Next()
		b.WriteString(" src=")
		b.WriteString(filepath.Base(frame.File))
		b.WriteByte(':')
		b.WriteString(strconv.Itoa(frame.Line))
	}

	b.WriteString(" msg=")
	writeLogfmtValue(&b, r.Message)

	r.Attrs(func(a slog.Attr) bool {
		b.WriteByte(' ')
		b.WriteString(a.Key)
		b.WriteByte('=')
		writeLogfmtValue(&b, a.Value.String())
		return true
	})
	b.WriteByte('\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := io.WriteString(h.w, b.String())
	return err
}

func (h *lineHandler) WithAttrs(attrs []slog.Attr) slog.Handler { return h }
func (h *lineHandler) WithGroup(name string) slog.Handler       { return h }

// writeLogfmtValue quotes s only when it needs it, to keep lines short.
// Anything that could split or spoof a line — whitespace, control
// characters, quotes, "=" — forces quoting (which escapes them), because
// values such as upload filenames and CSV cells are user-controlled.
func writeLogfmtValue(b *strings.Builder, s string) {
	if strings.IndexFunc(s, needsQuoting) >= 0 {
		b.WriteString(strconv.Quote(s))
		return
	}
	b.WriteString(s)
}

func needsQuoting(r rune) bool {
	return r <= ' ' || r == 0x7f || r == '"' || r == '=' || r == '\\'
}

var (
	level   = new(slog.LevelVar) // default LevelDebug, set below
	handler atomic.Pointer[slog.Handler]
)

func init() {
	level.Set(slog.LevelDebug)
	SetOutput(os.Stdout)
}

// SetLevel sets the minimum level that gets written.
func SetLevel(l slog.Level) {
	level.Set(l)
}

// SetOutput redirects log output (default os.Stdout).
func SetOutput(w io.Writer) {
	var h slog.Handler = &lineHandler{w: w, level: level}
	handler.Store(&h)
}

// Logger is a context bound to the package logger, obtained via
// WithCtx. It exists so every log line can carry the trace ID (and any
// other context-scoped fields) without repeating ctx on every call.
type Logger struct{ ctx context.Context }

// WithCtx binds ctx for the log calls that follow, e.g.
// logs.WithCtx(r.Context()).Warn("rate limit exceeded", "ip", ip).
func WithCtx(ctx context.Context) Logger { return Logger{ctx: ctx} }

// log builds the slog.Record by hand — rather than going through
// slog.Logger — so the captured source location is the caller of
// Debug/Info/Warn/Error, not a frame inside this package.
func (l Logger) log(lvl slog.Level, msg string, args ...any) {
	h := *handler.Load()
	if !h.Enabled(l.ctx, lvl) {
		return
	}
	var pcs [1]uintptr
	runtime.Callers(3, pcs[:]) // skip [Callers, log, Debug/Info/Warn/Error]
	r := slog.NewRecord(time.Now(), lvl, msg, pcs[0])
	r.Add(args...)
	_ = h.Handle(l.ctx, r)
}

func (l Logger) Debug(msg string, args ...any) { l.log(slog.LevelDebug, msg, args...) }
func (l Logger) Info(msg string, args ...any)  { l.log(slog.LevelInfo, msg, args...) }
func (l Logger) Warn(msg string, args ...any)  { l.log(slog.LevelWarn, msg, args...) }
func (l Logger) Error(msg string, args ...any) { l.log(slog.LevelError, msg, args...) }
