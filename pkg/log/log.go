package log

import (
	"io"
	"os"

	"github.com/mattn/go-isatty"
	"github.com/rs/zerolog"
)

var (
	isTerminal           = isatty.IsTerminal(os.Stdout.Fd())
	Output     io.Writer = os.Stderr
)

func init() {
	zerolog.TimeFieldFormat = "2006-01-02 15:04:05"
}

func New(name string) zerolog.Logger {
	if isTerminal {
		return zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout}).
			With().
			Timestamp().
			Str("component", name).
			Logger()
	}
	return zerolog.New(Output).
		With().
		Str("component", name).
		Timestamp().
		Logger()
}
