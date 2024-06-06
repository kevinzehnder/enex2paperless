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

type ColorTheme struct {
	Background termenv.Color
	Foreground termenv.Color
	Red        termenv.Color
	Green      termenv.Color
	Yellow     termenv.Color
	Blue       termenv.Color
	Magenta    termenv.Color
	Cyan       termenv.Color
	White      termenv.Color
}

var solarizedLight = ColorTheme{
	Background: termenv.ColorProfile().Color("#fdf6e3"),
	Foreground: termenv.ColorProfile().Color("#657b83"),
	Red:        termenv.ColorProfile().Color("#dc322f"),
	Green:      termenv.ColorProfile().Color("#859900"),
	Yellow:     termenv.ColorProfile().Color("#b58900"),
	Blue:       termenv.ColorProfile().Color("#268bd2"),
	Magenta:    termenv.ColorProfile().Color("#d33682"),
	Cyan:       termenv.ColorProfile().Color("#2aa198"),
	White:      termenv.ColorProfile().Color("#eee8d5"),
}

var tokyoNight = ColorTheme{
	Background: termenv.ColorProfile().Color("#1a1b26"),
	Foreground: termenv.ColorProfile().Color("#c0caf5"),
	Red:        termenv.ColorProfile().Color("#f7768e"),
	Green:      termenv.ColorProfile().Color("#9ece6a"),
	Yellow:     termenv.ColorProfile().Color("#e0af68"),
	Blue:       termenv.ColorProfile().Color("#7aa2f7"),
	Magenta:    termenv.ColorProfile().Color("#bb9af7"),
	Cyan:       termenv.ColorProfile().Color("#7dcfff"),
	White:      termenv.ColorProfile().Color("#a9b1d6"),
}

var (
	timeFormat = "[15:04:05.000]"
	output     = termenv.NewOutput(os.Stdout)
)

type Handler struct {
	h     slog.Handler
	b     *bytes.Buffer
	m     *sync.Mutex
	theme ColorTheme
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

func colorize(color termenv.Color, text string) string {
	return output.String(text).Foreground(color).String()
}

func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	// generate log level string
	level := fmt.Sprintf("%5s", r.Level.String())
	switch r.Level {
	case slog.LevelDebug:
		level = colorize(h.theme.Blue, level)
	case slog.LevelInfo:
		level = colorize(h.theme.Green, level)
	case slog.LevelWarn:
		level = colorize(h.theme.Yellow, level)
	case slog.LevelError:
		level = colorize(h.theme.Red, level)
	}

	// prepare attrs string
	var attrStr string
	attrs, err := h.computeAttrs(ctx, r)
	if err != nil {
		return err
	}
	if attrs != nil {
		bytes, err := json.Marshal(attrs)
		if err != nil {
			return fmt.Errorf("error when marshaling attrs: %w", err)
		}
		attrStr = colorize(h.theme.Magenta, string(bytes)) // log attributes
	}

	// print log message
	fmt.Print(output.String(fmt.Sprintf("%v [%s] %s %s\n",
		colorize(h.theme.Yellow, r.Time.Format(timeFormat)), // log time
		level,                                   // log level
		colorize(h.theme.Foreground, r.Message), // log message
		attrStr,                                 // log attributes
	),
	))

	return nil
}

func NewHandler(opts *slog.HandlerOptions) *Handler {
	// if no opts are given, set default values
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}

	// create a buffer
	b := &bytes.Buffer{}

	// select color theme
	var theme ColorTheme
	if output.HasDarkBackground() {
		theme = tokyoNight
	} else {
		theme = solarizedLight
	}

	return &Handler{
		b: b,
		h: slog.NewJSONHandler(b, &slog.HandlerOptions{
			Level:       opts.Level,
			AddSource:   opts.AddSource,
			ReplaceAttr: suppressDefaults(opts.ReplaceAttr),
		}),
		m:     &sync.Mutex{},
		theme: theme,
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
