// Package logger provides a pretty, human-readable slog handler for all services.
// Format: HH:MM:SS  LEVEL  [service]  message  key=value ...
// This replaces the default JSON handler so docker logs are readable without jq.
package logger

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

var levelLabel = map[slog.Level]string{
	slog.LevelDebug: "DEBUG",
	slog.LevelInfo:  "INFO ",
	slog.LevelWarn:  "WARN ",
	slog.LevelError: "ERROR",
}

// New returns a pretty slog.Logger and sets it as the global slog default.
// service is included on every line (e.g. "api", "harvester", "analytic").
// levelStr matches the values used in config: "debug", "info", "warn", "error".
func New(service, levelStr string) *slog.Logger {
	l := slog.New(&prettyHandler{
		w:       os.Stdout,
		minLvl:  parseLevel(levelStr),
		service: service,
	})
	slog.SetDefault(l)
	return l
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

type prettyHandler struct {
	w        io.Writer
	minLvl   slog.Level
	service  string
	preAttrs string // pre-formatted key=value from WithAttrs
	prefix   string // group prefix from WithGroup
}

func (h *prettyHandler) Enabled(_ context.Context, l slog.Level) bool {
	return l >= h.minLvl
}

func (h *prettyHandler) Handle(_ context.Context, r slog.Record) error {
	label, ok := levelLabel[r.Level]
	if !ok {
		label = fmt.Sprintf("%-5s", r.Level.String())
	}

	var buf bytes.Buffer
	buf.WriteString(r.Time.Format("15:04:05"))
	buf.WriteString("  ")
	buf.WriteString(label)
	buf.WriteString("  [")
	buf.WriteString(h.service)
	buf.WriteString("]  ")
	buf.WriteString(r.Message)

	if h.preAttrs != "" {
		buf.WriteString("  ")
		buf.WriteString(h.preAttrs)
	}

	r.Attrs(func(a slog.Attr) bool {
		buf.WriteString("  ")
		buf.WriteString(h.prefix)
		buf.WriteString(a.Key)
		buf.WriteByte('=')
		v := a.Value.Any()
		if e, ok := v.(error); ok {
			buf.WriteString(e.Error())
		} else {
			fmt.Fprintf(&buf, "%v", v)
		}
		return true
	})

	buf.WriteByte('\n')
	_, err := h.w.Write(buf.Bytes())
	return err
}

func (h *prettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	var extra strings.Builder
	if h.preAttrs != "" {
		extra.WriteString(h.preAttrs)
	}
	for _, a := range attrs {
		if extra.Len() > 0 {
			extra.WriteString("  ")
		}
		extra.WriteString(h.prefix + a.Key)
		extra.WriteByte('=')
		v := a.Value.Any()
		if e, ok := v.(error); ok {
			extra.WriteString(e.Error())
		} else {
			fmt.Fprintf(&extra, "%v", v)
		}
	}
	return &prettyHandler{
		w:        h.w,
		minLvl:   h.minLvl,
		service:  h.service,
		preAttrs: extra.String(),
		prefix:   h.prefix,
	}
}

func (h *prettyHandler) WithGroup(name string) slog.Handler {
	return &prettyHandler{
		w:        h.w,
		minLvl:   h.minLvl,
		service:  h.service,
		preAttrs: h.preAttrs,
		prefix:   h.prefix + name + ".",
	}
}
