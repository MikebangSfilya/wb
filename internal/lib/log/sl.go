package sl

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
)

const (
	EnvLocal = "local"
	EnvDev   = "dev"
	EnvProd  = "prod"
)

func SetupLogger(env string) *slog.Logger {
	var l *slog.Logger

	switch env {
	case EnvLocal:
		opts := PrettyHandlerOptions{
			SlogOpts: slog.HandlerOptions{
				Level: slog.LevelDebug,
			},
		}
		handler := NewPrettyHandler(os.Stdout, opts)
		l = slog.New(handler)

	case EnvDev, EnvProd:
		l = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level:     slog.LevelInfo,
			AddSource: true,
		}))
	default:
		l = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
	}

	return l
}

type PrettyHandlerOptions struct {
	SlogOpts slog.HandlerOptions
}

type PrettyHandler struct {
	opts PrettyHandlerOptions
	l    *log.Logger
}

func NewPrettyHandler(out io.Writer, opts PrettyHandlerOptions) *PrettyHandler {
	return &PrettyHandler{
		opts: opts,
		l:    log.New(out, "", 0),
	}
}

func (h *PrettyHandler) Handle(_ context.Context, r slog.Record) error {
	level := r.Level.String()
	msg := r.Message

	const (
		Reset   = "\033[0m"
		Red     = "\033[31m"
		Green   = "\033[32m"
		Yellow  = "\033[33m"
		Magenta = "\033[35m"
		Cyan    = "\033[36m"
		Gray    = "\033[90m"
	)

	switch r.Level {
	case slog.LevelDebug:
		level = Magenta + level + Reset
	case slog.LevelInfo:
		level = Green + level + Reset
	case slog.LevelWarn:
		level = Yellow + level + Reset
	case slog.LevelError:
		level = Red + level + Reset
	}

	fields := make(map[string]interface{}, r.NumAttrs())
	r.Attrs(func(a slog.Attr) bool {
		val := a.Value.Any()
		if err, ok := val.(error); ok {
			fields[a.Key] = err.Error()
		} else {
			fields[a.Key] = val
		}
		return true
	})

	attrsStr := ""
	if len(fields) > 0 {
		b, err := json.Marshal(fields)
		if err == nil {
			attrsStr = Cyan + string(b) + Reset
		}
	}

	sourceStr := ""
	if r.Level != slog.LevelInfo {
		fs := runtime.CallersFrames([]uintptr{r.PC})
		f, _ := fs.Next()
		if f.File != "" {
			sourceStr = Gray + "(" + filepath.Base(f.File) + ":" + strconv.Itoa(f.Line) + ")" + Reset
		}
	}

	timeStr := r.Time.Format("15:04:05.000")

	h.l.Printf("%s %s %s %s %s", timeStr, level, msg, attrsStr, sourceStr)

	return nil
}

func (h *PrettyHandler) Enabled(_ context.Context, level slog.Level) bool {
	minLevel := slog.LevelInfo
	if h.opts.SlogOpts.Level != nil {
		minLevel = h.opts.SlogOpts.Level.Level()
	}
	return level >= minLevel
}

func (h *PrettyHandler) WithAttrs(_ []slog.Attr) slog.Handler {
	return h
}

func (h *PrettyHandler) WithGroup(_ string) slog.Handler {
	return h
}
