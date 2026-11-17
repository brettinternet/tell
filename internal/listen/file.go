package listen

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

// errMalformedInput reports that at least one --events line was rejected after
// every valid line had been processed.
var errMalformedInput = errors.New("input contained malformed envelopes")

var envelopeKeys = map[string]bool{"schema_version": true, "source": true, "type": true, "id": true, "received_at": true, "data": true}

type inputLine struct {
	number int
	data   []byte
	reason string
	err    error
}

// fileSpec admits exported envelopes as the only source. It opens no connection
// and binds nothing; reading ends the capture at EOF.
func fileSpec(r io.Reader, malformed *bool) sourceSpec {
	return sourceSpec{name: "file", prepare: func() (func(context.Context, *admission, func(Envelope, string)) error, error) {
		return func(ctx context.Context, a *admission, receipt func(Envelope, string)) error {
			lines := make(chan inputLine)
			// A blocked stdin read cannot observe cancellation; the reader is
			// abandoned on interrupt and the process exits.
			go readLines(ctx, r, lines)
			for {
				var line inputLine
				select {
				case <-ctx.Done():
					return nil
				case line = <-lines:
				}
				if line.err != nil {
					if line.err == io.EOF {
						return nil
					}
					return line.err
				}
				if line.reason != "" {
					*malformed = true
					receipt(Envelope{}, fmt.Sprintf("not_sent/%s line=%d", line.reason, line.number))
					continue
				}
				e, encoded, reason := storedEnvelope(line.data)
				if reason != "" {
					*malformed = true
					receipt(Envelope{}, fmt.Sprintf("not_sent/%s line=%d", reason, line.number))
					continue
				}
				var digest *[32]byte
				if e.Source == "github" {
					// Live GitHub admission also suppresses an identical body
					// under a new delivery ID; data is that body.
					body, _ := json.Marshal(e.Data)
					sum := sha256.Sum256(body)
					digest = &sum
				}
				receipt(e, a.admitWait(ctx, e, encoded, digest, time.Now()))
			}
		}, nil
	}}
}

// readLines splits NDJSON without buffering more than one envelope; an
// oversized line is discarded up to its newline and reported by number.
func readLines(ctx context.Context, r io.Reader, lines chan<- inputLine) {
	reader := bufio.NewReaderSize(r, envelopeLimit+1)
	send := func(line inputLine) bool {
		select {
		case lines <- line:
			return true
		case <-ctx.Done():
			return false
		}
	}
	for number := 1; ; number++ {
		data, err := reader.ReadSlice('\n')
		oversized := false
		for err == bufio.ErrBufferFull {
			oversized = true
			_, err = reader.ReadSlice('\n')
		}
		if len(data) > 0 && data[len(data)-1] == '\n' {
			data = data[:len(data)-1]
		}
		switch {
		case oversized:
			if !send(inputLine{number: number, reason: "event_size"}) {
				return
			}
		case len(data) > 0:
			if !send(inputLine{number: number, data: append([]byte(nil), data...)}) {
				return
			}
		}
		if err != nil {
			send(inputLine{err: err})
			return
		}
	}
}

// storedEnvelope applies the live envelope rules to one exported line.
func storedEnvelope(line []byte) (Envelope, []byte, string) {
	if len(line) > envelopeLimit {
		return Envelope{}, nil, "event_size"
	}
	fields, err := object(line)
	if err != nil {
		return Envelope{}, nil, "invalid_json"
	}
	if len(fields) != len(envelopeKeys) {
		return Envelope{}, nil, "invalid_envelope"
	}
	for key := range fields {
		if !envelopeKeys[key] {
			return Envelope{}, nil, "invalid_envelope"
		}
	}
	var e Envelope
	if json.Unmarshal(line, &e) != nil || e.SchemaVersion != 1 || (e.Source != "slack" && e.Source != "github") || e.Data == nil {
		return Envelope{}, nil, "invalid_envelope"
	}
	if !metadata(e.ID, 256) || !metadata(e.Type, 128) {
		return Envelope{}, nil, "invalid_identity"
	}
	if _, err := time.Parse(time.RFC3339Nano, e.ReceivedAt); err != nil {
		return Envelope{}, nil, "invalid_envelope"
	}
	encoded, err := json.Marshal(e)
	if err != nil || len(encoded) > envelopeLimit {
		return Envelope{}, nil, "event_size"
	}
	return e, encoded, ""
}
