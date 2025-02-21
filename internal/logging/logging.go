package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"

	"github.com/muesli/termenv"
)

type Handler struct {
	h       slog.Handler
	b       *bytes.Buffer
	m       *sync.Mutex
	output  *termenv.Output
	nocolor bool
}

func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.h.Enabled(ctx, level)
}

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Handler{h: h.h.WithAttrs(attrs), b: h.b, m: h.m}
}

func (h *Handler) WithGroup(name string) slog.Handler {
	return &Handler{h: h.h.WithGroup(name), b: h.b, m: h.m}
}

func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	// prepare log level string
	level := fmt.Sprintf("%5s", r.Level.String())
	p := h.output.ColorProfile()
	if !h.nocolor {
		switch r.Level {
		case slog.LevelDebug:
			level = h.output.String(level).Foreground(p.Color("12")).String()
		case slog.LevelInfo:
			level = h.output.String(level).Foreground(p.Color("10")).String()
		case slog.LevelWarn:
			level = h.output.String(level).Foreground(p.Color("11")).String()
		case slog.LevelError:
			level = h.output.String(level).Foreground(p.Color("9")).String()
		}
	}

	// prepare attrs string
	attrs, err := h.computeAttrs(ctx, r)
	if err != nil {
		return err
	}

	attrStr := ""
	if attrs != nil {
		bytes, err := json.Marshal(attrs)
		if err != nil {
			return fmt.Errorf("error when marshaling attrs: %w", err)
		}
		if h.nocolor {
			attrStr = string(bytes)
		} else {
			attrStr = h.output.String(string(bytes)).Foreground(p.Color("13")).String()
		}
	}

	// prepare time string
	timeStr := r.Time.Format("[15:04:05.000]")
	if !h.nocolor {
		timeStr = h.output.String(timeStr).Foreground(p.Color("11")).String()
	}

	// print log message
	fmt.Printf("%s [%s] %s %s\n",
		timeStr,
		level,
		r.Message,
		attrStr,
	)

	return nil
}

func NewHandler(opts *slog.HandlerOptions, nocolor bool) *Handler {
	output := termenv.NewOutput(os.Stdout)

	// if no opts are given, set default values
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}

	// create a buffer
	b := &bytes.Buffer{}

	// If terminal doesn't support ANSI256 or TrueColor, force nocolor
	profile := output.ColorProfile()
	if !(profile == termenv.TrueColor || profile == termenv.ANSI256) {
		nocolor = true
	}

	return &Handler{
		b: b,
		h: slog.NewJSONHandler(b, &slog.HandlerOptions{
			Level:       opts.Level,
			AddSource:   opts.AddSource,
			ReplaceAttr: suppressDefaults(opts.ReplaceAttr),
		}),
		m:       &sync.Mutex{},
		output:  output,
		nocolor: nocolor,
	}
}

func suppressDefaults(
	next func([]string, slog.Attr) slog.Attr,
) func([]string, slog.Attr) slog.Attr {
	return func(groups []string, a slog.Attr) slog.Attr {
		if a.Key == slog.TimeKey ||
			a.Key == slog.LevelKey ||
			a.Key == slog.MessageKey {
			return slog.Attr{}
		}
		if next == nil {
			return a
		}
		return next(groups, a)
	}
}

func (h *Handler) computeAttrs(
	ctx context.Context,
	r slog.Record,
) (map[string]any, error) {
	h.m.Lock()
	defer func() {
		h.b.Reset()
		h.m.Unlock()
	}()

	err := h.h.Handle(ctx, r)
	if err != nil {
		return nil, fmt.Errorf("error when calling inner handler's Handle: %w", err)
	}

	var attrs map[string]any
	err = json.Unmarshal(h.b.Bytes(), &attrs)
	if err != nil {
		return nil, fmt.Errorf("error when unmarshaling inner handler's Handle result: %w", err)
	}

	if len(attrs) == 0 {
		return nil, nil
	}

	return attrs, nil
}
