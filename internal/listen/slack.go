package listen

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
)

// SlackOptions contains invocation-owned streams and credentials, never printable configuration.
type SlackOptions struct {
	Token    string
	Timeout  time.Duration
	Out, Err io.Writer
	OnEvent  func(Envelope, func(route, outcome, reason string), io.Writer)
	OnStop   func()
}

type SourceOptions struct {
	Name          string
	Token         string
	Secret        string
	Addr          string
	RepositoryIDs []int64
	Timeout       time.Duration
}

type CaptureOptions struct {
	Sources  []SourceOptions
	Out, Err io.Writer
	OnEvent  func(Envelope, func(route, outcome, reason string), io.Writer)
	OnStop   func()
}

type sourceSpec struct {
	name    string
	prepare func() (func(context.Context, *admission, func(Envelope, string)) error, error)
}
type transport struct {
	endpoint      string
	client        *http.Client
	dial          func(context.Context, string) (*websocket.Conn, error)
	wait          func(context.Context, time.Duration) bool
	now           func() time.Time
	jitter        func(time.Duration) time.Duration
	outputTimeout time.Duration
}

func production() transport {
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return transport{
		endpoint: "https://slack.com/api/apps.connections.open", client: client,
		dial: func(ctx context.Context, address string) (*websocket.Conn, error) {
			u, err := url.Parse(address)
			if err != nil || u.Scheme != "wss" || u.Host == "" || u.User != nil {
				return nil, &setupError{reason: "invalid_connection_url"}
			}
			conn, response, err := websocket.Dial(ctx, address, &websocket.DialOptions{HTTPClient: client})
			if err != nil && response != nil {
				if response.StatusCode == http.StatusTooManyRequests {
					return nil, &setupError{reason: "rate_limited", retry: true, after: retryAfter(response.Header.Get("Retry-After"), time.Now())}
				}
				if response.StatusCode >= 400 && response.StatusCode < 500 {
					return nil, &setupError{reason: "websocket_setup_failed"}
				}
			}
			return conn, err
		}, wait: wait, now: time.Now, jitter: func(d time.Duration) time.Duration { return d + time.Duration(rand.Int64N(int64(d/5)+1)) }, outputTimeout: 5 * time.Second,
	}
}
func wait(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// Slack captures until cancellation or a fatal source/output failure. One blocked
// writer goroutine per stream is the maximum; process exit releases blocked OS IO.
// Neither the provider reader nor acknowledgments wait for these streams.
func Slack(ctx context.Context, opts SlackOptions) error { return run(ctx, opts, production()) }
func run(ctx context.Context, opts SlackOptions, tr transport) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
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
	receipt := func(e Envelope, outcome string) {
		queueDiagnostic := func(line string) {
			select {
			case diagnostics <- &outputRequest{data: []byte(line)}:
			default:
				fail(errors.New("listener diagnostic output busy"))
			}
		}
		line := fmt.Sprintf("tell listen: source=slack outcome=%s\n", outcome)
		if e.ID != "" {
			line = fmt.Sprintf("tell listen: source=slack id=%q type=%q outcome=%s\n", e.ID, e.Type, outcome)
		}
		queueDiagnostic(line)
	}
	actionReceipt := func(e Envelope, route, outcome, reason string) {
		line := fmt.Sprintf("tell listen: source=slack id=%q type=%q route=%q outcome=%s", e.ID, e.Type, route, outcome)
		if reason != "" {
			line += " reason=" + reason
		}
		select {
		case diagnostics <- &outputRequest{data: []byte(line + "\n")}:
		default:
			fail(errors.New("listener diagnostic output busy"))
		}
	}
	outputDone := make(chan struct{}, 2)
	eventsDone := make(chan struct{})
	if opts.OnEvent == nil {
		go func() { outputBytes(outputCtx, opts.Out, a.events, tr.outputTimeout, fail); outputDone <- struct{}{} }()
	} else {
		deliveryWriter := &queuedWriter{queue: diagnostics, fail: fail, ctx: outputCtx}
		go func() {
			handleEvents(a.events, opts.OnEvent, actionReceipt, deliveryWriter, fail)
			close(eventsDone)
			outputDone <- struct{}{}
		}()
	}
	go func() { output(outputCtx, opts.Err, diagnostics, tr.outputTimeout, fail); outputDone <- struct{}{} }()
	sourceDone := make(chan error, 1)
	go func() { sourceDone <- receive(ctx, opts, tr, a, receipt) }()
	var result error
	select {
	case <-ctx.Done():
		cancel()
		<-sourceDone
	case result = <-failures:
		cancel()
		<-sourceDone
	case result = <-sourceDone:
		if ctx.Err() != nil {
			result = nil
		}
	}
	// Stop admission first, then finish bounded writes. This preserves stopped
	// receipts and prevents successful writes racing the caller after return.
	cancel()
	if result != nil {
		receipt(Envelope{}, "failed reason="+strconv.Quote(result.Error()))
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
func handleEvents(queue <-chan []byte, handler func(Envelope, func(route, outcome, reason string), io.Writer), report func(Envelope, string, string, string), deliveryWriter io.Writer, fail func(error)) {
	for data := range queue {
		var event Envelope
		if json.Unmarshal(data, &event) != nil {
			fail(errors.New("listener event queue contained invalid data"))
			return
		}
		handler(event, func(route, outcome, reason string) { report(event, route, outcome, reason) }, deliveryWriter)
	}
}

const (
	outputPending uint32 = iota
	outputStarted
	outputExpired
)

type outputRequest struct {
	data  []byte
	ctx   context.Context
	ack   chan outputResult
	state atomic.Uint32
	once  sync.Once
}

type outputResult struct {
	err     error
	started bool
}

type outputCompletion struct {
	request *outputRequest
	result  outputResult
}

var errOutputStalled = errors.New("listener output stalled")

type queuedWriteFailure struct {
	outcome string
	reason  string
	cause   error
}

func (err *queuedWriteFailure) Error() string { return err.cause.Error() }
func (err *queuedWriteFailure) Unwrap() error { return err.cause }

func (request *outputRequest) respond(result outputResult) {
	if request.ack != nil {
		request.once.Do(func() { request.ack <- result })
	}
}

func (request *outputRequest) skipExpired() bool {
	if request.state.Load() == outputExpired {
		return true
	}
	if request.ctx == nil || request.ctx.Err() == nil {
		return false
	}
	if request.state.CompareAndSwap(outputPending, outputExpired) {
		request.respond(outputResult{err: request.ctx.Err()})
		return true
	}
	return request.state.Load() == outputExpired
}

func (request *outputRequest) started() bool { return request.state.Load() == outputStarted }

func (request *outputRequest) expire(err error) {
	if request.state.CompareAndSwap(outputPending, outputExpired) {
		request.respond(outputResult{err: err})
		return
	}
	request.respond(outputResult{err: err, started: request.started()})
}

type queuedWriter struct {
	queue chan<- *outputRequest
	fail  func(error)
	ctx   context.Context
}

func (writer *queuedWriter) Write(data []byte) (int, error) {
	return writer.WriteContext(context.Background(), data)
}

func (writer *queuedWriter) WriteContext(ctx context.Context, data []byte) (int, error) {
	request := &outputRequest{data: append([]byte(nil), data...), ctx: ctx, ack: make(chan outputResult, 1)}
	var writerDone <-chan struct{}
	if writer.ctx != nil {
		writerDone = writer.ctx.Done()
	}
	select {
	case writer.queue <- request:
	case <-ctx.Done():
		return 0, queuedWriteError(outputResult{err: ctx.Err()})
	case <-writerDone:
		return 0, queuedWriteError(outputResult{err: context.Canceled})
	default:
		writer.fail(errors.New("listener diagnostic output busy"))
		return 0, errors.New("listener diagnostic output busy")
	}
	select {
	case result := <-request.ack:
		return queuedWriteResult(len(data), result)
	case <-ctx.Done():
		request.expire(ctx.Err())
		return queuedWriteResultOnCancellation(len(data), request, ctx.Err())
	case <-writerDone:
		request.expire(context.Canceled)
		return queuedWriteResultOnCancellation(len(data), request, context.Canceled)
	}
}

func queuedWriteResultOnCancellation(size int, request *outputRequest, err error) (int, error) {
	select {
	case result := <-request.ack:
		return queuedWriteResult(size, result)
	default:
		return 0, queuedWriteError(outputResult{err: err, started: request.started()})
	}
}

func queuedWriteResult(size int, result outputResult) (int, error) {
	if result.err != nil {
		return 0, queuedWriteError(result)
	}
	return size, nil
}

func queuedWriteError(result outputResult) error {
	if !errors.Is(result.err, context.Canceled) && !errors.Is(result.err, context.DeadlineExceeded) && !errors.Is(result.err, errOutputStalled) {
		return result.err
	}
	reason := "cancelled"
	if errors.Is(result.err, context.DeadlineExceeded) {
		reason = "timeout"
	} else if errors.Is(result.err, errOutputStalled) {
		reason = "output_stalled"
	}
	outcome := "not_sent"
	if result.started {
		outcome = "uncertain"
	}
	return &queuedWriteFailure{outcome: outcome, reason: reason, cause: result.err}
}

func output(ctx context.Context, writer io.Writer, queue <-chan *outputRequest, timeout time.Duration, fail func(error)) {
	// A dedicated writer bounds the number of blocked Writes, even for arbitrary
	// io.Writers without deadlines. Never spawn another after a timeout.
	jobs := make(chan *outputRequest)
	defer close(jobs)
	done := make(chan outputCompletion, 1)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case request, ok := <-jobs:
				if !ok {
					return
				}
				if request.skipExpired() || !request.state.CompareAndSwap(outputPending, outputStarted) {
					done <- outputCompletion{request: request, result: outputResult{err: context.Canceled}}
					continue
				}
				if request.ctx != nil && request.ctx.Err() != nil {
					request.state.CompareAndSwap(outputStarted, outputExpired)
					request.respond(outputResult{err: request.ctx.Err()})
					done <- outputCompletion{request: request, result: outputResult{err: request.ctx.Err()}}
					continue
				}
				n, err := writer.Write(request.data)
				if err == nil && n != len(request.data) {
					err = io.ErrShortWrite
				}
				result := outputResult{err: err, started: true}
				request.respond(result)
				select {
				case done <- outputCompletion{request: request, result: result}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case request, ok := <-queue:
			if !ok {
				return
			}
			if request.skipExpired() {
				continue
			}
			requestContext := request.ctx
			var requestDone <-chan struct{}
			if requestContext != nil {
				requestDone = requestContext.Done()
			}
			select {
			case jobs <- request:
			case <-ctx.Done():
				request.expire(ctx.Err())
				return
			case <-requestDone:
				request.skipExpired()
				continue
			}
			timer := time.NewTimer(timeout)
		awaitWrite:
			for {
				select {
				case <-ctx.Done():
					timer.Stop()
					request.expire(ctx.Err())
					return
				case <-requestDone:
					request.expire(requestContext.Err())
					requestDone = nil
				case completed := <-done:
					timer.Stop()
					if completed.request != request {
						fail(errors.New("listener output writer lost request order"))
						return
					}
					if completed.result.err != nil {
						if !completed.result.started && request.skipExpired() {
							break awaitWrite
						}
						fail(errors.New("listener output failed"))
						return
					}
					break awaitWrite
				case <-timer.C:
					request.expire(errOutputStalled)
					fail(errOutputStalled)
					return
				}
			}
		}
	}
}

func outputBytes(ctx context.Context, writer io.Writer, queue <-chan []byte, timeout time.Duration, fail func(error)) {
	// A dedicated writer bounds the number of blocked Writes, even for arbitrary
	// io.Writers without deadlines. Never spawn another after a timeout.
	jobs := make(chan []byte)
	defer close(jobs)
	done := make(chan error, 1)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case data, ok := <-jobs:
				if !ok {
					return
				}
				n, err := writer.Write(data)
				if err == nil && n != len(data) {
					err = io.ErrShortWrite
				}
				select {
				case done <- err:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case data, ok := <-queue:
			if !ok {
				return
			}
			select {
			case jobs <- data:
			case <-ctx.Done():
				return
			}
			timer := time.NewTimer(timeout)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case err := <-done:
				timer.Stop()
				if err != nil {
					fail(errors.New("listener output failed"))
					return
				}
			case <-timer.C:
				fail(errors.New("listener output stalled"))
				return
			}
		}
	}
}

type setupError struct {
	reason string
	retry  bool
	after  time.Duration
}

func (e *setupError) Error() string { return "Slack setup: " + e.reason }
func open(ctx context.Context, opts SlackOptions, tr transport) (string, *setupError) {
	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tr.endpoint, nil)
	if err != nil {
		return "", &setupError{reason: "invalid_setup"}
	}
	req.Header.Set("Authorization", "Bearer "+opts.Token)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := tr.client.Do(req)
	if err != nil {
		return "", &setupError{reason: "network", retry: true}
	}
	defer resp.Body.Close()
	if resp.StatusCode == 429 {
		return "", &setupError{reason: "rate_limited", retry: true, after: retryAfter(resp.Header.Get("Retry-After"), tr.now())}
	}
	if resp.StatusCode >= 500 {
		return "", &setupError{reason: "server_error", retry: true}
	}
	if resp.StatusCode != 200 {
		return "", &setupError{reason: "http_error"}
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, rawLimit+1))
	if err != nil {
		return "", &setupError{reason: "network", retry: true}
	}
	data, err := object(raw)
	if err != nil {
		return "", &setupError{reason: "invalid_response"}
	}
	var ok bool
	_ = json.Unmarshal(data["ok"], &ok)
	if !ok {
		reason := field(data, "error")
		switch reason {
		case "invalid_auth", "not_authed", "token_revoked", "account_inactive", "missing_scope", "not_allowed_token_type", "app_not_enabled":
		case "ratelimited":
			return "", &setupError{reason: "rate_limited", retry: true, after: retryAfter(resp.Header.Get("Retry-After"), tr.now())}
		case "internal_error", "fatal_error", "service_unavailable":
			return "", &setupError{reason: "server_error", retry: true}
		default:
			reason = "setup_failed"
		}
		return "", &setupError{reason: reason}
	}
	address := field(data, "url")
	if address == "" {
		return "", &setupError{reason: "invalid_response"}
	}
	return address, nil
}
func retryAfter(value string, now time.Time) time.Duration {
	if seconds, err := strconv.ParseUint(value, 10, 32); err == nil {
		return time.Duration(seconds) * time.Second
	}
	if date, err := http.ParseTime(value); err == nil {
		return date.Sub(now)
	}
	return 0
}
func receive(ctx context.Context, opts SlackOptions, tr transport, a *admission, receipt func(Envelope, string)) error {
	defer receipt(Envelope{}, "stopped")
	delay := time.Second
	for ctx.Err() == nil {
		receipt(Envelope{}, "connecting")
		address, failure := open(ctx, opts, tr)
		retryAfter := time.Duration(0)
		if failure != nil {
			if !failure.retry {
				return failure
			}
			retryAfter = failure.after
		} else {
			setupCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
			conn, err := tr.dial(setupCtx, address)
			cancel()
			if err == nil {
				started := tr.now()
				receipt(Envelope{}, "connected")
				err = connection(ctx, conn, tr, a, receipt)
				conn.CloseNow()
				if err != nil {
					return err
				}
				if tr.now().Sub(started) >= time.Minute {
					delay = time.Second
				}
			} else {
				var setup *setupError
				if errors.As(err, &setup) {
					if !setup.retry {
						return setup
					}
					retryAfter = setup.after
				}
			}
		}
		if ctx.Err() != nil {
			break
		}
		receipt(Envelope{}, "reconnecting")
		pause := tr.jitter(delay)
		if pause > time.Minute {
			pause = time.Minute
		}
		if retryAfter > pause {
			pause = retryAfter
		}
		if !tr.wait(ctx, pause) {
			break
		}
		delay *= 2
		if delay > time.Minute {
			delay = time.Minute
		}
	}
	return nil
}
func connection(ctx context.Context, conn *websocket.Conn, tr transport, a *admission, receipt func(Envelope, string)) error {
	conn.SetReadLimit(rawLimit)
	for {
		kind, raw, err := conn.Read(ctx)
		if err != nil {
			return nil
		}
		if kind != websocket.MessageText {
			receipt(Envelope{}, "not_sent/invalid_frame")
			continue
		}
		message, err := frame(raw)
		if err != nil {
			receipt(Envelope{}, "not_sent/invalid_json")
			continue
		}
		if id := field(message, "envelope_id"); metadata(id, 256) {
			ack, _ := json.Marshal(map[string]string{"envelope_id": id})
			ackCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			err = conn.Write(ackCtx, websocket.MessageText, ack)
			cancel()
			if err != nil {
				return nil
			}
		} else if field(message, "type") != "hello" && field(message, "type") != "disconnect" {
			receipt(Envelope{}, "not_sent/invalid_identity")
			continue
		}
		switch field(message, "type") {
		case "hello":
			continue
		case "disconnect":
			if field(message, "reason") == "link_disabled" {
				return errors.New("Slack disconnected: link_disabled")
			}
			return nil
		case "events_api":
			e, encoded, err := event(message["payload"], tr.now())
			if errors.Is(err, errIgnored) {
				receipt(Envelope{}, "ignored")
				continue
			}
			if err != nil {
				receipt(Envelope{}, "not_sent/"+err.Error())
				continue
			}
			receipt(e, a.admit(e, encoded, tr.now()))
		default:
			receipt(Envelope{}, "ignored")
		}
	}
}
