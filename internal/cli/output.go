package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
	urfave "github.com/urfave/cli/v3"
)

// Style only text owned by the human CLI, never a delivery/protocol writer.
// Each destination is checked separately so redirecting stdout leaves stderr usable.
type outputStyle bool

func newOutputStyle(w io.Writer, plain bool) outputStyle {
	if plain || os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	file, ok := w.(interface{ Fd() uintptr })
	return outputStyle(ok && isatty.IsTerminal(file.Fd()))
}

func commandStyle(cmd *urfave.Command, w io.Writer) outputStyle {
	plain, _ := cmd.Root().Metadata["plain-output"].(bool)
	return newOutputStyle(w, plain || cmd.Bool("no-color") || cmd.Bool("json"))
}

func (s outputStyle) paint(code, text string) string {
	if !s {
		return text
	}
	return "\x1b[" + code + "m" + text + "\x1b[0m"
}

func (s outputStyle) bold(text string) string    { return s.paint("1", text) }
func (s outputStyle) success(text string) string { return s.paint("32", text) }
func (s outputStyle) warning(text string) string { return s.paint("33", text) }
func (s outputStyle) failure(text string) string { return s.paint("31", text) }

func (s outputStyle) outcome(outcome string) string {
	switch outcome {
	case "accepted":
		return s.success(outcome)
	case "uncertain":
		return s.warning(outcome)
	default:
		return s.failure(outcome)
	}
}

// Let urfave lay out plain help first; ANSI bytes must not affect column widths.
func printHelp(w io.Writer, templ string, data any) {
	cmd, ok := data.(*urfave.Command)
	if !ok {
		urfave.DefaultPrintHelp(w, templ, data)
		return
	}
	style := commandStyle(cmd, w)
	if !bool(style) {
		urfave.DefaultPrintHelp(w, templ, data)
		return
	}
	var rendered strings.Builder
	urfave.DefaultPrintHelp(&rendered, templ, data)
	for _, line := range strings.SplitAfter(rendered.String(), "\n") {
		heading := strings.TrimSuffix(line, "\n")
		if strings.HasSuffix(heading, ":") && !strings.HasPrefix(heading, " ") {
			line = style.bold(heading) + strings.TrimPrefix(line, heading)
		}
		fmt.Fprint(w, line)
	}
}
