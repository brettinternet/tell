package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	protocol "github.com/modelcontextprotocol/go-sdk/mcp"
)

// ServeStdio feeds requests to the SDK as they arrive and forwards responses
// independently, allowing the SDK to handle concurrent calls and cancellation.
// On EOF it waits for every request response before closing the SDK connection.
func ServeStdio(ctx context.Context, server *protocol.Server, input io.Reader, output io.Writer) error {
	requestReader, requestWriter := io.Pipe()
	responseReader, responseWriter := io.Pipe()
	session, err := server.Connect(ctx, &protocol.IOTransport{Reader: requestReader, Writer: responseWriter}, nil)
	if err != nil {
		_ = requestReader.Close()
		_ = requestWriter.Close()
		_ = responseReader.Close()
		_ = responseWriter.Close()
		return err
	}

	pending := newResponseTracker()
	inputDone := make(chan error, 1)
	go readRequests(input, requestWriter, pending, inputDone)

	outputDone := make(chan error, 1)
	go forwardResponses(responseReader, output, pending, outputDone)

	var serveErr error
	var outputErr error
	outputEnded := false
	waitForOutput := func() error {
		if outputEnded {
			return outputErr
		}
		outputErr = <-outputDone
		outputEnded = true
		return outputErr
	}
	closeInput := func() {
		if closer, ok := input.(io.Closer); ok {
			_ = closer.Close()
		}
	}

	select {
	case <-ctx.Done():
		serveErr = ctx.Err()
		closeInput()
		_ = requestWriter.CloseWithError(serveErr)
	case err := <-inputDone:
		if err != nil {
			serveErr = fmt.Errorf("read MCP request: %w", err)
			_ = requestWriter.CloseWithError(err)
			break
		}
		for {
			changed, empty := pending.state()
			if empty {
				_ = requestWriter.Close()
				break
			}
			select {
			case <-changed:
			case <-ctx.Done():
				serveErr = ctx.Err()
				closeInput()
				_ = requestWriter.CloseWithError(serveErr)
			case err := <-outputDone:
				outputEnded = true
				outputErr = err
				if err != nil {
					serveErr = err
				} else {
					serveErr = errors.New("MCP server closed before responding to all requests")
				}
				closeInput()
				_ = requestWriter.CloseWithError(serveErr)
			}
			if serveErr != nil {
				break
			}
			if outputEnded {
				break
			}
		}
	case err := <-outputDone:
		outputEnded = true
		outputErr = err
		if err != nil {
			serveErr = err
		} else {
			serveErr = errors.New("MCP server closed before stdin reached EOF")
		}
		closeInput()
		_ = requestWriter.CloseWithError(serveErr)
	}

	waitErr := session.Wait()
	_ = requestReader.Close()
	_ = requestWriter.Close()
	if !outputEnded {
		outputErr = waitForOutput()
	}
	_ = responseReader.Close()
	_ = responseWriter.Close()
	if outputErr != nil {
		outputErr = fmt.Errorf("forward MCP response: %w", outputErr)
	}
	return errors.Join(serveErr, waitErr, outputErr)
}

func readRequests(input io.Reader, writer *io.PipeWriter, pending *responseTracker, done chan<- error) {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64*1024), protocol.DefaultMaxLineLength)
	for scanner.Scan() {
		// The SDK's own decoder skips inter-message whitespace; match it so a
		// blank keepalive line does not end the session.
		if len(bytes.TrimSpace(scanner.Bytes())) == 0 {
			continue
		}
		line := append([]byte(nil), scanner.Bytes()...)
		ids, err := requestIDs(line)
		if err != nil {
			done <- err
			return
		}
		pending.add(ids)
		if _, err := writer.Write(append(line, '\n')); err != nil {
			done <- err
			return
		}
	}
	done <- scanner.Err()
}

func forwardResponses(reader io.Reader, output io.Writer, pending *responseTracker, done chan<- error) {
	fail := func(err error) {
		if closer, ok := reader.(io.Closer); ok {
			_ = closer.Close()
		}
		done <- err
	}
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), protocol.DefaultMaxLineLength)
	for scanner.Scan() {
		line := append([]byte(nil), scanner.Bytes()...)
		ids, safeLine, err := sanitizeResponseLine(line)
		if err != nil {
			fail(fmt.Errorf("decode MCP response: %w", err))
			return
		}
		if _, err := output.Write(append(safeLine, '\n')); err != nil {
			fail(fmt.Errorf("write MCP response: %w", err))
			return
		}
		pending.complete(ids)
	}
	if err := scanner.Err(); err != nil {
		fail(err)
		return
	}
	done <- nil
}

func requestIDs(line []byte) ([]jsonrpc.ID, error) {
	rawMessages, _, err := splitMessages(line)
	if err != nil {
		return nil, errors.New("invalid MCP request")
	}
	messages, err := decodeMessages(rawMessages)
	if err != nil {
		return nil, errors.New("invalid MCP request")
	}
	var ids []jsonrpc.ID
	for _, message := range messages {
		if request, ok := message.(*jsonrpc.Request); ok && request.ID.IsValid() {
			ids = append(ids, request.ID)
		}
	}
	return ids, nil
}

func sanitizeResponseLine(line []byte) ([]jsonrpc.ID, []byte, error) {
	rawMessages, batch, err := splitMessages(line)
	if err != nil {
		return nil, nil, errors.New("invalid MCP response")
	}
	messages, err := decodeMessages(rawMessages)
	if err != nil {
		return nil, nil, errors.New("invalid MCP response")
	}
	var ids []jsonrpc.ID
	changed := false
	for i, message := range messages {
		response, ok := message.(*jsonrpc.Response)
		if !ok {
			continue
		}
		if response.ID.IsValid() {
			ids = append(ids, response.ID)
		}
		if response.Error == nil {
			continue
		}
		var fields struct {
			JSONRPC json.RawMessage `json:"jsonrpc"`
			ID      json.RawMessage `json:"id"`
			Error   struct {
				Code json.RawMessage `json:"code"`
			} `json:"error"`
		}
		if err := json.Unmarshal(rawMessages[i], &fields); err != nil || len(fields.JSONRPC) == 0 || len(fields.ID) == 0 || len(fields.Error.Code) == 0 {
			return nil, nil, errors.New("invalid MCP response")
		}
		safeError, err := json.Marshal(struct {
			Code    json.RawMessage `json:"code"`
			Message string          `json:"message"`
		}{fields.Error.Code, "request could not be processed"})
		if err != nil {
			return nil, nil, errors.New("invalid MCP response")
		}
		safeResponse, err := json.Marshal(struct {
			JSONRPC json.RawMessage `json:"jsonrpc"`
			ID      json.RawMessage `json:"id"`
			Error   json.RawMessage `json:"error"`
		}{fields.JSONRPC, fields.ID, safeError})
		if err != nil {
			return nil, nil, errors.New("invalid MCP response")
		}
		rawMessages[i] = safeResponse
		changed = true
	}
	if !changed {
		return ids, line, nil
	}
	if !batch {
		return ids, rawMessages[0], nil
	}
	safeLine, err := json.Marshal(rawMessages)
	if err != nil {
		return nil, nil, errors.New("invalid MCP response")
	}
	return ids, safeLine, nil
}

func decodeMessages(rawMessages []json.RawMessage) ([]jsonrpc.Message, error) {
	messages := make([]jsonrpc.Message, 0, len(rawMessages))
	for _, raw := range rawMessages {
		message, err := jsonrpc.DecodeMessage(raw)
		if err != nil {
			return nil, errors.New("invalid JSON-RPC message")
		}
		messages = append(messages, message)
	}
	return messages, nil
}

func splitMessages(line []byte) ([]json.RawMessage, bool, error) {
	trimmed := bytes.TrimSpace(line)
	if len(trimmed) == 0 {
		return nil, false, errors.New("empty protocol line")
	}
	if trimmed[0] == '[' {
		var rawMessages []json.RawMessage
		if err := json.Unmarshal(trimmed, &rawMessages); err != nil {
			return nil, false, errors.New("invalid protocol batch")
		}
		if len(rawMessages) == 0 {
			return nil, false, errors.New("empty protocol batch")
		}
		return rawMessages, true, nil
	}
	return []json.RawMessage{trimmed}, false, nil
}

type responseTracker struct {
	mu      sync.Mutex
	pending map[string]int
	changed chan struct{}
}

func newResponseTracker() *responseTracker {
	return &responseTracker{pending: make(map[string]int), changed: make(chan struct{})}
}

func (p *responseTracker) add(ids []jsonrpc.ID) {
	if len(ids) == 0 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, id := range ids {
		key := idKey(id)
		p.pending[key]++
	}
	p.signalLocked()
}

func (p *responseTracker) complete(ids []jsonrpc.ID) {
	if len(ids) == 0 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, id := range ids {
		key := idKey(id)
		if p.pending[key] <= 1 {
			delete(p.pending, key)
		} else {
			p.pending[key]--
		}
	}
	p.signalLocked()
}

func (p *responseTracker) state() (<-chan struct{}, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.changed, len(p.pending) == 0
}

func (p *responseTracker) signalLocked() {
	close(p.changed)
	p.changed = make(chan struct{})
}

func idKey(id jsonrpc.ID) string {
	encoded, err := json.Marshal(id.Raw())
	if err != nil {
		panic(fmt.Sprintf("marshal SDK JSON-RPC ID: %v", err))
	}
	return string(encoded)
}
