package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
)

func StackArrow(err error) string {
	if err == nil {
		return ""
	}

	var chain []string
	for e := err; e != nil; e = errors.Unwrap(e) {
		parts := strings.SplitN(e.Error(), ": ", 2)
		chain = append(chain, parts[0])
	}

	return strings.Join(chain, " -> ")
}

type LogHandler struct {
	threadPad int
	level     slog.Level
	out       io.Writer
}

func (h *LogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *LogHandler) Handle(_ context.Context, r slog.Record) error {
	src := "???"
	if r.PC != 0 {
		s := r.Source()
		src = fmt.Sprintf("%s:%d %s",
			filepath.Base(s.File),
			s.Line,
			filepath.Base(s.Function))
	}

	t := r.Time.Format("2006/01/02 15:04:05")

	levelStr := map[slog.Level]string{
		slog.LevelDebug: "DEBUG",
		slog.LevelInfo:  "INFO ",
		slog.LevelWarn:  "WARN ",
		slog.LevelError: "ERROR",
	}[r.Level]

	var threadText string
	var errText string
	otherAttrs := make([]string, 0)

	r.Attrs(func(a slog.Attr) bool {
		switch a.Key {
		case "thread":
			threadText = a.Value.String()
		case "error":
			e, ok := a.Value.Any().(error)
			if ok {
				errText = StackArrow(e)
			} else {
				otherAttrs = append(otherAttrs, fmt.Sprintf("%s=%v", a.Key, a.Value.Any()))
			}
		default:
			otherAttrs = append(otherAttrs, fmt.Sprintf("%s=%v", a.Key, a.Value.Any()))
		}
		return true
	})

	attrText := ""
	if len(otherAttrs) > 0 {
		attrText = " " + strings.Join(otherAttrs, " ")
	}

	if len(threadText) < h.threadPad {
		threadText += strings.Repeat(" ", h.threadPad-len(threadText))
	}
	if errText != "" {
		fmt.Fprintf(h.out, "[%s] [%s /%s] - %s | %s | error=[%s] (%s)\n",
			t, threadText, levelStr, r.Message, attrText, errText, src)
	} else {
		fmt.Fprintf(h.out, "[%s] [%s /%s] - %s | %s (%s) \n",
			t, threadText, levelStr, r.Message, attrText, src)
	}

	return nil
}

func (h *LogHandler) WithAttrs(attrs []slog.Attr) slog.Handler { return h }
func (h *LogHandler) WithGroup(name string) slog.Handler       { return h }
