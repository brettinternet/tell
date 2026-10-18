package config

import (
	"errors"
	"net"
	"strconv"
	"strings"
)

// SlackTokenEnv returns a name, never its credential value.
func (cfg Config) SlackTokenEnv() string {
	if cfg.Listen.Slack == nil {
		return ""
	}
	return cfg.Listen.Slack.AppTokenEnv
}

// GitHubSecretEnv returns a name, never its credential value.
func (cfg Config) GitHubSecretEnv() string {
	if cfg.Listen.GitHub == nil {
		return ""
	}
	return cfg.Listen.GitHub.SecretEnv
}

// HTTPAddr returns the safe loopback default even when no override is set.
func (cfg Config) HTTPAddr() string {
	if cfg.Listen.HTTP == nil || cfg.Listen.HTTP.Addr == "" {
		return defaultListenHTTPAddr
	}
	return cfg.Listen.HTTP.Addr
}

func repositoryIDs(source *GitHub) string {
	if source == nil {
		return ""
	}
	values := make([]string, len(source.RepositoryIDs))
	for i, id := range source.RepositoryIDs {
		values[i] = strconv.FormatInt(id, 10)
	}
	return strings.Join(values, ",")
}

func validateListenHTTPAddr(addr string) error {
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

// ChildEnv strips every named inbound credential even from unrelated outbound providers.
func (cfg Config) ChildEnv(env []string) []string {
	secrets := map[string]bool{}
	if name := cfg.SlackTokenEnv(); name != "" {
		secrets[name] = true
	}
	if name := cfg.GitHubSecretEnv(); name != "" {
		secrets[name] = true
	}
	result := make([]string, 0, len(env))
	for _, entry := range env {
		name, _, _ := strings.Cut(entry, "=")
		if !secrets[name] {
			result = append(result, entry)
		}
	}
	return result
}
