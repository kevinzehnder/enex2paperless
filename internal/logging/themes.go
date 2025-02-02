package logging

import "github.com/muesli/termenv"

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
