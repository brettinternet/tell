package delivery

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/nicholas-fedor/shoutrrr/pkg/router"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

func init() {
	// A few Shoutrrr services warn through the standard library logger, whose
	// text can include configuration. Tell never uses it, so discard it globally.
	log.SetOutput(io.Discard)
}

var discardLogger types.StdLogger = log.New(io.Discard, "", 0)

// Shoutrrr URLs are credentials. Its errors and logs can embed them, so results
// carry only fixed messages chosen by errors.Is/As, never library text.
func shoutrrrRouter(urlsEnv string, timeout time.Duration) (*router.ServiceRouter, string) {
	value, ok := os.LookupEnv(urlsEnv)
	urls := strings.Fields(value)
	if !ok || len(urls) == 0 {
		return nil, "missing_urls"
	}
	// Construction parses every URL and builds services without network I/O.
	sender, err := router.NewWithOptions(discardLogger, types.SenderOptions{Timeout: timeout}, urls...)
	if err != nil {
		return nil, "invalid_config"
	}
	return sender, ""
}

func notifyShoutrrr(ctx context.Context, timeout time.Duration, urlsEnv string, message Message) *Result {
	const route = "shoutrrr"
	sender, reason := shoutrrrRouter(urlsEnv, timeout)
	switch reason {
	case "missing_urls":
		return NewResult(route, "not_sent", reason, "Shoutrrr URL environment variable is unset or empty")
	case "invalid_config":
		return NewResult(route, "not_sent", reason, "Shoutrrr URLs are invalid or name an unknown service; run tell doctor --via shoutrrr")
	}
	defer sender.Close()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	params := types.Params{}
	params.SetTitle(message.Title)
	var failed error
	for _, err := range sender.SendContext(ctx, message.Body, &params) {
		if err == nil {
			continue
		}
		// With several URLs some may have been delivered, so a timeout anywhere
		// makes the whole step uncertain rather than failed.
		if errors.Is(err, router.ErrServiceTimeout) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			reason := "cancelled"
			if !errors.Is(err, context.Canceled) {
				reason = "timeout"
			}
			return NewResult(route, "uncertain", reason, "Shoutrrr send was interrupted; delivery may have occurred. Check delivery before retrying")
		}
		if failed == nil {
			failed = err
		}
	}
	if failed == nil {
		return NewResult(route, "accepted", "", "")
	}
	return NewResult(route, "failed", "provider_failure", "Shoutrrr reported failure"+shoutrrrDiagnostic(failed))
}

func shoutrrrDiagnostic(err error) string {
	var unknownAuthority x509.UnknownAuthorityError
	var hostname x509.HostnameError
	var invalid x509.CertificateInvalidError
	var verification *tls.CertificateVerificationError
	var dns *net.DNSError
	var operation *net.OpError
	hint := ""
	switch {
	case errors.As(err, &unknownAuthority), errors.As(err, &hostname), errors.As(err, &invalid), errors.As(err, &verification):
		hint = "; diagnostic hint: TLS certificate error; check the provider certificate and local trust store"
	case errors.As(err, &dns), errors.As(err, &operation):
		hint = "; diagnostic hint: connection error; check network access and the provider endpoint"
	}
	return hint + "; run tell doctor --via shoutrrr. With several URLs some may have been delivered; check delivery before retrying"
}
