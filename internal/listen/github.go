package listen

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	githubPath         = "/github"
	githubHeaderLimit  = 16 << 10
	githubReaderLimit  = 16
	githubReadTimeout  = 5 * time.Second
	githubWriteTimeout = 5 * time.Second
	githubIdleTimeout  = 30 * time.Second
)

var githubEventPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,127}$`)
var githubDeliveryPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func githubSpec(source SourceOptions) (sourceSpec, error) {
	if source.Secret == "" || len(source.RepositoryIDs) == 0 {
		return sourceSpec{}, errors.New("GitHub webhook secret and repository allowlist are required")
	}
	if err := validLoopbackAddr(source.Addr); err != nil {
		return sourceSpec{}, err
	}
	allowed := make(map[int64]bool, len(source.RepositoryIDs))
	for _, id := range source.RepositoryIDs {
		if id <= 0 || allowed[id] {
			return sourceSpec{}, errors.New("GitHub repository allowlist must contain unique positive IDs")
		}
		allowed[id] = true
	}
	return sourceSpec{name: "github", prepare: func() (func(context.Context, *admission, func(Envelope, string)) error, error) {
		listener, err := net.Listen("tcp", source.Addr)
		if err != nil {
			return nil, errors.New("GitHub loopback HTTP bind failed")
		}
		return func(ctx context.Context, a *admission, receipt func(Envelope, string)) error {
			return serveGitHub(ctx, listener, source.Secret, allowed, a, receipt)
		}, nil
	}}, nil
}

func validLoopbackAddr(addr string) error {
	host, portText, err := net.SplitHostPort(addr)
	if err != nil {
		return errors.New("listen.http.addr must be a literal loopback IP and port")
	}
	ip := net.ParseIP(host)
	port, portErr := strconv.ParseUint(portText, 10, 16)
	if ip == nil || !ip.IsLoopback() || portErr != nil || port == 0 || strconv.FormatUint(port, 10) != portText {
		return errors.New("listen.http.addr must be a literal loopback IP and port from 1 to 65535")
	}
	return nil
}

func serveGitHub(ctx context.Context, listener net.Listener, secret string, allowed map[int64]bool, admission *admission, receipt func(Envelope, string)) error {
	receipt(Envelope{}, "connecting")
	readers := make(chan struct{}, githubReaderLimit)
	server := &http.Server{
		Handler:                      githubHandler(secret, allowed, admission, receipt, readers),
		DisableGeneralOptionsHandler: true,
		ReadHeaderTimeout:            githubReadTimeout,
		ReadTimeout:                  githubReadTimeout,
		WriteTimeout:                 githubWriteTimeout,
		IdleTimeout:                  githubIdleTimeout,
		MaxHeaderBytes:               githubHeaderLimit,
		ErrorLog:                     log.New(io.Discard, "", 0),
	}
	server.SetKeepAlivesEnabled(false)
	done := make(chan error, 1)
	go func() { done <- server.Serve(&headerLimitListener{Listener: listener}) }()
	receipt(Envelope{}, "connected")
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), githubReadTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
		}
		<-done
		receipt(Envelope{}, "stopped")
		return nil
	case err := <-done:
		receipt(Envelope{}, "stopped")
		if errors.Is(err, http.ErrServerClosed) || ctx.Err() != nil {
			return nil
		}
		return errors.New("GitHub loopback HTTP source failed")
	}
}

type headerLimitListener struct{ net.Listener }

func (listener *headerLimitListener) Accept() (net.Conn, error) {
	connection, err := listener.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return &headerLimitConn{Conn: connection}, nil
}

type headerLimitConn struct {
	net.Conn
	header     []byte
	pending    []byte
	complete   bool
	rejected   bool
	pendingErr error
}

func (connection *headerLimitConn) Read(buffer []byte) (int, error) {
	if len(buffer) == 0 {
		return 0, nil
	}
	if len(connection.pending) > 0 {
		return connection.readPending(buffer)
	}
	if connection.rejected {
		return 0, io.EOF
	}
	if connection.complete {
		return connection.Conn.Read(buffer)
	}
	for {
		readSize := len(buffer)
		if remaining := githubHeaderLimit + 1 - len(connection.header); readSize > remaining {
			readSize = remaining
		}
		chunk := make([]byte, readSize)
		n, err := connection.Conn.Read(chunk)
		if n > 0 {
			connection.header = append(connection.header, chunk[:n]...)
			if end := bytes.Index(connection.header, []byte("\r\n\r\n")); end >= 0 {
				if end+4 > githubHeaderLimit {
					return connection.rejectHeader()
				}
				connection.complete = true
				connection.pending = connection.header
				connection.header = nil
				connection.pendingErr = err
				return connection.readPending(buffer)
			}
			if len(connection.header) > githubHeaderLimit {
				return connection.rejectHeader()
			}
		}
		if err != nil {
			return 0, err
		}
	}
}

func (connection *headerLimitConn) readPending(buffer []byte) (int, error) {
	n := copy(buffer, connection.pending)
	connection.pending = connection.pending[n:]
	if len(connection.pending) == 0 && connection.pendingErr != nil {
		err := connection.pendingErr
		connection.pendingErr = nil
		return n, err
	}
	return n, nil
}

func (connection *headerLimitConn) rejectHeader() (int, error) {
	connection.rejected = true
	_, _ = io.WriteString(connection.Conn, "HTTP/1.1 431 Request Header Fields Too Large\r\nContent-Length: 0\r\nConnection: close\r\n\r\n")
	return 0, io.EOF
}

func githubHandler(secret string, allowed map[int64]bool, admission *admission, receipt func(Envelope, string), readers chan struct{}) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != githubPath || r.URL.EscapedPath() != githubPath {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !githubContentType(r.Header) || !githubIdentityEncoding(r.Header) {
			w.WriteHeader(http.StatusUnsupportedMediaType)
			return
		}
		select {
		case readers <- struct{}{}:
			defer func() { <-readers }()
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if admission.isClosed() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		deliveryID, ok := uniqueHeader(r.Header, "X-GitHub-Delivery")
		if !ok || !githubDeliveryPattern.MatchString(deliveryID) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		eventType, ok := uniqueHeader(r.Header, "X-GitHub-Event")
		if !ok || !githubEventPattern.MatchString(eventType) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		signature, present, ambiguous := identityHeader(r.Header, "X-Hub-Signature-256")
		if ambiguous {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if !present {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		provided, ok := githubSignature(signature)
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.ContentLength > rawLimit {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		raw, err := io.ReadAll(io.LimitReader(r.Body, rawLimit+1))
		if err != nil {
			if admission.isClosed() || r.Context().Err() != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
			} else {
				w.WriteHeader(http.StatusBadRequest)
			}
			return
		}
		if len(raw) > rawLimit {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		mac := hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write(raw)
		if !hmac.Equal(provided, mac.Sum(nil)) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if admission.isClosed() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		event, encoded, repositoryID, err := githubEnvelope(raw, eventType, deliveryID, time.Now())
		if err != nil {
			if errors.Is(err, errEnvelopeTooLarge) {
				w.WriteHeader(http.StatusRequestEntityTooLarge)
			} else {
				w.WriteHeader(http.StatusBadRequest)
			}
			return
		}
		if !allowed[repositoryID] {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if eventType == "ping" {
			if admission.isClosed() {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			receipt(event, "ignored")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		digest := sha256.Sum256(raw)
		switch admission.admitGitHub(event, encoded, digest, time.Now()) {
		case "accepted":
			receipt(event, "accepted")
			w.WriteHeader(http.StatusAccepted)
		case "duplicate":
			receipt(event, "duplicate")
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	})
}

func githubContentType(header http.Header) bool {
	value, ok := singleValue(header, "Content-Type")
	if !ok {
		return false
	}
	mediaType, params, err := mime.ParseMediaType(value)
	if err != nil || mediaType != "application/json" {
		return false
	}
	for key, value := range params {
		if key != "charset" || !strings.EqualFold(value, "utf-8") && !strings.EqualFold(value, "utf8") {
			return false
		}
	}
	return true
}

func githubIdentityEncoding(header http.Header) bool {
	values := header.Values("Content-Encoding")
	if len(values) == 0 {
		return true
	}
	return len(values) == 1 && !strings.Contains(values[0], ",") && strings.EqualFold(strings.TrimSpace(values[0]), "identity")
}

func uniqueHeader(header http.Header, name string) (string, bool) {
	value, ok := singleValue(header, name)
	if !ok || strings.Contains(value, ",") {
		return "", false
	}
	return value, true
}

func singleValue(header http.Header, name string) (string, bool) {
	values := header.Values(name)
	if len(values) != 1 {
		return "", false
	}
	return strings.TrimSpace(values[0]), true
}

func identityHeader(header http.Header, name string) (value string, present, ambiguous bool) {
	values := header.Values(name)
	if len(values) == 0 {
		return "", false, false
	}
	if len(values) != 1 || strings.Contains(values[0], ",") {
		return "", true, true
	}
	return strings.TrimSpace(values[0]), true, false
}

func githubSignature(value string) ([]byte, bool) {
	if !strings.HasPrefix(value, "sha256=") || len(value) != len("sha256=")+sha256.Size*2 {
		return nil, false
	}
	decoded, err := hex.DecodeString(strings.TrimPrefix(value, "sha256="))
	return decoded, err == nil && len(decoded) == sha256.Size
}

var errEnvelopeTooLarge = errors.New("envelope too large")

func githubEnvelope(raw []byte, eventType, deliveryID string, now time.Time) (Envelope, []byte, int64, error) {
	if !utf8.Valid(raw) {
		return Envelope{}, nil, 0, errors.New("invalid JSON")
	}
	body, err := object(raw)
	if err != nil {
		return Envelope{}, nil, 0, err
	}
	var repository map[string]json.RawMessage
	if json.Unmarshal(body["repository"], &repository) != nil || repository == nil {
		return Envelope{}, nil, 0, errors.New("invalid repository")
	}
	repositoryID, err := strconv.ParseInt(string(bytes.TrimSpace(repository["id"])), 10, 64)
	if err != nil || repositoryID <= 0 {
		return Envelope{}, nil, 0, errors.New("invalid repository ID")
	}
	event := Envelope{
		SchemaVersion: 1,
		Source:        "github",
		Type:          eventType,
		ID:            deliveryID,
		ReceivedAt:    now.UTC().Format("2006-01-02T15:04:05.000Z"),
		Data:          body,
	}
	encoded, err := json.Marshal(event)
	if err != nil {
		return Envelope{}, nil, 0, err
	}
	if len(encoded) > envelopeLimit {
		return Envelope{}, nil, 0, errEnvelopeTooLarge
	}
	return event, append(encoded, '\n'), repositoryID, nil
}
