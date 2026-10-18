package listen

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/brettinternet/tell/internal/config"
	deliverypkg "github.com/brettinternet/tell/internal/delivery"
)

const actionWorkers = 4
const actionQueueLimit = 64

type action struct {
	route    config.Route
	event    Envelope
	envelope []byte
	report   func(route, outcome, reason string)
}

// Executor bounds active and waiting command actions. It never retries work.
type Executor struct {
	cfg     config.Config
	cwd     string
	timeout time.Duration
	jobs    chan action
	stop    chan struct{}
	ctx     context.Context
	cancel  context.CancelFunc
	mu      sync.Mutex
	stopped bool
	workers sync.WaitGroup
	done    chan struct{}
}

func NewExecutor(cfg config.Config, cwd string) *Executor {
	timeout, _ := time.ParseDuration(cfg.Timeout)
	ctx, cancel := context.WithCancel(context.Background())
	e := &Executor{
		cfg: cfg, cwd: cwd, timeout: timeout,
		jobs: make(chan action, actionQueueLimit), stop: make(chan struct{}),
		ctx: ctx, cancel: cancel, done: make(chan struct{}),
	}
	for range actionWorkers {
		e.workers.Add(1)
		go e.worker()
	}
	go func() {
		e.workers.Wait()
		close(e.done)
	}()
	return e
}

// Submit enqueues one route action without waiting for a running child.
func (e *Executor) Submit(route config.Route, event Envelope, report func(route, outcome, reason string)) bool {
	envelope, err := json.Marshal(event)
	if err != nil {
		if report != nil {
			report(route.Name, "failed", "invalid_event")
		}
		return true
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.stopped {
		return false
	}
	select {
	case e.jobs <- action{route: route, event: event, envelope: envelope, report: report}:
		return true
	default:
		return false
	}
}

func (e *Executor) worker() {
	defer e.workers.Done()
	for {
		select {
		case <-e.stop:
			return
		case task := <-e.jobs:
			e.mu.Lock()
			stopped := e.stopped
			e.mu.Unlock()
			if stopped {
				reportAction(task, "not_sent", "shutdown")
				continue
			}
			e.execute(task)
		}
	}
}

func (e *Executor) execute(task action) {
	argv := task.route.Run
	env := commandEnvironment(e.cfg, task.route.Name, task.event)
	code, failure := deliverypkg.RunCommand(
		e.ctx, e.timeout, "listen/"+task.route.Name, argv[0], argv[1:],
		bytes.NewReader(task.envelope), env, e.cwd,
	)
	if failure != nil {
		reportAction(task, failure.Outcome, failure.Failure.Reason)
		return
	}
	switch code {
	case 0:
		reportAction(task, "accepted", "")
	case 3:
		reportAction(task, "not_sent", "no_match")
	default:
		reportAction(task, "failed", "provider_failure")
	}
}

func commandEnvironment(cfg config.Config, route string, event Envelope) []string {
	fixed := map[string]string{
		"TELL_EVENT_SOURCE": event.Source,
		"TELL_EVENT_TYPE":   event.Type,
		"TELL_EVENT_ID":     event.ID,
		"TELL_ROUTE":        route,
		"TELL_PROJECT":      cfg.Project,
	}
	reserved := map[string]bool{}
	for name := range fixed {
		reserved[name] = true
	}
	inherited := cfg.ChildEnv(os.Environ())
	filtered := make([]string, 0, len(inherited)+len(fixed))
	for _, entry := range inherited {
		name, _, _ := strings.Cut(entry, "=")
		if !reserved[name] {
			filtered = append(filtered, entry)
		}
	}
	for _, name := range []string{"TELL_EVENT_SOURCE", "TELL_EVENT_TYPE", "TELL_EVENT_ID", "TELL_ROUTE", "TELL_PROJECT"} {
		filtered = append(filtered, name+"="+fixed[name])
	}
	return filtered
}

func reportAction(task action, outcome, reason string) {
	if task.report != nil {
		task.report(task.route.Name, outcome, reason)
	}
}

// Stop stops admission and drops all queued actions. Actions already assigned to
// workers continue until their own timeout unless CancelRunning is called.
func (e *Executor) Stop() {
	e.mu.Lock()
	if e.stopped {
		e.mu.Unlock()
		return
	}
	e.stopped = true
	close(e.stop)
	var dropped []action
	for {
		select {
		case task := <-e.jobs:
			dropped = append(dropped, task)
		default:
			e.mu.Unlock()
			for _, task := range dropped {
				reportAction(task, "not_sent", "shutdown")
			}
			return
		}
	}
}

// CancelRunning marks interrupted started actions uncertain through the shared
// subprocess outcome path. It is used only for the listener's second signal.
func (e *Executor) CancelRunning() { e.cancel() }

func (e *Executor) Wait() { <-e.done }
