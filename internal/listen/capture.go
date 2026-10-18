package listen

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Capture starts selected sources with one shared ingress buffer and dispatcher.
// Preparation (including loopback binds) completes before any provider connects.
func Capture(ctx context.Context, opts CaptureOptions) error {
	if len(opts.Sources) == 0 {
		return errors.New("no listener sources selected")
	}
	var specs []sourceSpec
	seen := map[string]bool{}
	for _, source := range opts.Sources {
		if seen[source.Name] {
			return errors.New("duplicate listener source")
		}
		seen[source.Name] = true
		switch source.Name {
		case "slack":
			source, tr := source, production()
			specs = append(specs, sourceSpec{name: "slack", prepare: func() (func(context.Context, *admission, func(Envelope, string)) error, error) {
				return func(ctx context.Context, a *admission, receipt func(Envelope, string)) error {
					return receive(ctx, SlackOptions{Token: source.Token, Timeout: source.Timeout}, tr, a, receipt)
				}, nil
			}})
		case "github":
			spec, err := githubSpec(source)
			if err != nil {
				return err
			}
			specs = append(specs, spec)
		default:
			return errors.New("unknown listener source")
		}
	}
	return runSources(ctx, opts, specs)
}

func runSources(ctx context.Context, opts CaptureOptions, specs []sourceSpec) error {
	sourceCtx, cancelSources := context.WithCancel(ctx)
	defer cancelSources()
	a := newAdmission()
	diagnostics := make(chan *outputRequest, 64)
	outputCtx, stopOutput := context.WithCancel(context.Background())
	defer stopOutput()
	failures := make(chan error, 1)
	fail := func(err error) {
		stopOutput()
		select {
		case failures <- err:
		default:
		}
	}
	queueDiagnostic := func(line string) {
		select {
		case diagnostics <- &outputRequest{data: []byte(line)}:
		default:
			fail(errors.New("listener diagnostic output busy"))
		}
	}
	receipts := make(map[string]func(Envelope, string), len(specs))
	for _, spec := range specs {
		name := spec.name
		receipts[name] = func(e Envelope, outcome string) {
			line := fmt.Sprintf("tell listen: source=%s outcome=%s\n", name, outcome)
			if e.ID != "" {
				line = fmt.Sprintf("tell listen: source=%s id=%q type=%q outcome=%s\n", name, e.ID, e.Type, outcome)
			}
			queueDiagnostic(line)
		}
	}
	actionReceipt := func(e Envelope, route, outcome, reason string) {
		line := fmt.Sprintf("tell listen: source=%s id=%q type=%q route=%q outcome=%s", e.Source, e.ID, e.Type, route, outcome)
		if reason != "" {
			line += " reason=" + reason
		}
		queueDiagnostic(line + "\n")
	}
	outputDone := make(chan struct{}, 2)
	eventsDone := make(chan struct{})
	if opts.OnEvent == nil {
		go func() {
			outputBytes(outputCtx, opts.Out, a.events, 5*time.Second, fail)
			outputDone <- struct{}{}
		}()
	} else {
		deliveryWriter := &queuedWriter{queue: diagnostics, fail: fail, ctx: outputCtx}
		go func() {
			handleEvents(a.events, opts.OnEvent, actionReceipt, deliveryWriter, fail)
			close(eventsDone)
			outputDone <- struct{}{}
		}()
	}
	go func() { output(outputCtx, opts.Err, diagnostics, 5*time.Second, fail); outputDone <- struct{}{} }()

	prepared := make([]func(context.Context, *admission, func(Envelope, string)) error, 0, len(specs))
	preparedNames := make([]string, 0, len(specs))
	var result error
	var failedSource string
	for _, spec := range specs {
		if ctx.Err() != nil {
			break
		}
		run, err := spec.prepare()
		if err != nil {
			failedSource = spec.name
			receipts[spec.name](Envelope{}, "failed reason=bind_failed")
			result = err
			break
		}
		prepared = append(prepared, run)
		preparedNames = append(preparedNames, spec.name)
	}
	if result != nil {
		prepared = nil
		preparedNames = nil
	}
	type sourceResult struct {
		name string
		err  error
	}
	sourceDone := make(chan sourceResult, len(prepared))
	for i, run := range prepared {
		receipt := receipts[preparedNames[i]]
		name := preparedNames[i]
		go func(run func(context.Context, *admission, func(Envelope, string)) error) {
			sourceDone <- sourceResult{name: name, err: run(sourceCtx, a, receipt)}
		}(run)
	}
	completed := 0
	if result == nil && len(prepared) > 0 {
		select {
		case <-ctx.Done():
		case result = <-failures:
		case source := <-sourceDone:
			completed++
			failedSource = source.name
			result = source.err
			if ctx.Err() != nil {
				result = nil
			}
		}
	}
	// Closing admission under its mutex makes shutdown races return 503 without
	// reserving an ID or digest, then cancellation closes every source.
	a.close()
	cancelSources()
	for ; completed < len(prepared); completed++ {
		source := <-sourceDone
		if result == nil && source.err != nil {
			result, failedSource = source.err, source.name
		}
	}
	if result != nil && failedSource != "" && len(prepared) > 0 {
		receipts[failedSource](Envelope{}, "failed reason=source_failed")
	}
	close(a.events)
	if opts.OnEvent != nil {
		<-eventsDone
		if opts.OnStop != nil {
			opts.OnStop()
		}
	}
	close(diagnostics)
	<-outputDone
	<-outputDone
	select {
	case err := <-failures:
		if result == nil {
			result = err
		}
	default:
	}
	return result
}
