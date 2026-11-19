package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/brettinternet/tell/internal/delivery"
	urfave "github.com/urfave/cli/v3"
	"golang.org/x/sys/unix"
)

const maxCommandDisplay = 120

func runCommand() *urfave.Command {
	first := 1
	return &urfave.Command{Name: "run", Usage: "Run a command, then notify when it finishes", ArgsUsage: "[--] COMMAND [ARG...]",
		Description:  "The command keeps the terminal and its exit status. Flags after COMMAND belong to it.\nCtrl-C stops the command without notifying.\n\nExamples:\n  tell run go test ./...\n  tell run --via desktop -t Deploy -- make deploy",
		StopOnNthArg: &first,
		Flags: []urfave.Flag{
			&urfave.StringFlag{Name: "title", Aliases: []string{"t"}, Usage: "Notification `title` (default: project directory name)"},
			levelFlag(),
		}, Action: runAndNotify}
}

func runAndNotify(ctx context.Context, cmd *urfave.Command) error {
	if cmd.Bool("json") {
		return &failure{Code: "usage", Message: "run does not support --json; the command owns stdout", usage: true}
	}
	if cmd.NArg() == 0 {
		return &failure{Code: "usage", Message: "run requires a command; run tell run --help", usage: true}
	}
	cfg, err := loadSettings(cmd)
	if err != nil {
		return err
	}
	if message := selectLevel(cmd, &cfg); message != "" {
		return &failure{Code: "usage", Message: message, usage: true}
	}
	args := cmd.Args().Slice()
	style := commandStyle(cmd, cmd.ErrWriter)

	// The terminal sends Ctrl-C to the whole foreground process group, so Tell
	// only records SIGINT; forwarding it would interrupt the command twice.
	signals := make(chan os.Signal, 4)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(signals)

	child := exec.Command(args[0], args[1:]...)
	child.Stdin, child.Stdout, child.Stderr = cmd.Reader, cmd.Writer, cmd.ErrWriter
	start := time.Now()
	if err := child.Start(); err != nil {
		name := delivery.RedactDiagnostic(args[0])
		if errors.Is(err, exec.ErrNotFound) || errors.Is(err, fs.ErrNotExist) {
			fmt.Fprintf(cmd.ErrWriter, "%s command not found: %s\n", style.failure("tell:"), name)
			return exitStatus(127)
		}
		reason := "cannot execute"
		if errors.Is(err, fs.ErrPermission) {
			reason = "permission denied"
		}
		fmt.Fprintf(cmd.ErrWriter, "%s %s: %s\n", style.failure("tell:"), reason, name)
		return exitStatus(126)
	}
	waited := make(chan struct{})
	go func() { _ = child.Wait(); close(waited) }()
	interrupted := false
wait:
	for {
		select {
		case sig := <-signals:
			if sig == os.Interrupt {
				interrupted = true
			} else {
				_ = child.Process.Signal(sig)
			}
		case <-waited:
			break wait
		}
	}
	elapsed := time.Since(start)
	if child.ProcessState == nil {
		return exitStatus(1)
	}
	status, ok := child.ProcessState.Sys().(syscall.WaitStatus)
	if !ok {
		return exitStatus(1)
	}
	code, outcome := status.ExitStatus(), "finished in "
	switch {
	case status.Signaled():
		code = 128 + int(status.Signal())
		outcome = "killed by " + unix.SignalName(status.Signal()) + " after "
	case code != 0:
		outcome = fmt.Sprintf("failed with exit %d after ", code)
	}
	if interrupted {
		return exitStatus(code)
	}

	// Main cancels ctx on the signal forwarded above; delivery still has its own
	// timeout, and any signal after the command exits abandons delivery.
	deliverCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	defer cancel()
	go func() {
		select {
		case <-signals:
			cancel()
		case <-deliverCtx.Done():
		}
	}()
	title := cmd.String("title")
	if !cmd.IsSet("title") {
		title = filepath.Base(cfg.Project)
	}
	kind := "success"
	if code != 0 {
		kind = "failure"
	}
	message := delivery.Message{Title: title, Body: commandLine(args) + " " + outcome + humanDuration(elapsed), Type: kind}
	if result := delivery.Notify(deliverCtx, cfg, message, cmd.ErrWriter); result.ExitCode() != 0 {
		fmt.Fprintf(cmd.ErrWriter, "%s %s\n", style.warning("tell:"), humanReason(result.Route, result.Failure.Reason, result.Error()))
	}
	return exitStatus(code)
}

// Quote like a shell so the notification shows which arguments were grouped.
func commandLine(args []string) string {
	words := make([]string, len(args))
	for i, arg := range args {
		plain := arg != "" && strings.IndexFunc(arg, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r) && !strings.ContainsRune("-_./:=,+@%", r)
		}) < 0
		words[i] = arg
		if !plain {
			words[i] = "'" + strings.ReplaceAll(arg, "'", `'\''`) + "'"
		}
	}
	line := strings.Join(words, " ")
	if utf8.RuneCountInString(line) > maxCommandDisplay {
		line = string([]rune(line)[:maxCommandDisplay-1]) + "…"
	}
	return line
}

func humanDuration(d time.Duration) string {
	var text string
	switch {
	case d < time.Second:
		text = d.Round(time.Millisecond).String()
	case d < time.Hour:
		text = d.Round(time.Second).String()
	default:
		text = d.Round(time.Minute).String()
	}
	text = strings.Replace(text, "m0s", "m", 1)
	return strings.Replace(text, "h0m", "h", 1)
}
