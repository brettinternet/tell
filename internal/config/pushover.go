package config

import (
	"errors"
	"regexp"
	"time"
)

// Pushover names the environment variables holding the app token and user
// key; the credentials never appear in configuration or diagnostics.
type Pushover struct {
	TokenEnv string `json:"token_env,omitempty"`
	UserEnv  string `json:"user_env,omitempty"`
	Priority string `json:"priority,omitempty"`
	Retry    string `json:"retry,omitempty"`
	Expire   string `json:"expire,omitempty"`
	Device   string `json:"device,omitempty"`
}

// PushoverPriorities maps Pushover priority names to API values.
var PushoverPriorities = map[string]string{"lowest": "-2", "low": "-1", "normal": "0", "high": "1", "emergency": "2"}

var pushoverDevice = regexp.MustCompile(`^[A-Za-z0-9_-]{1,25}(,[A-Za-z0-9_-]{1,25})*$`)

// applyPushoverOption validates one pushover key shared by the top-level block
// and cascade step options.
func applyPushoverOption(p *Pushover, key, value string) error {
	switch key {
	case "token_env", "user_env":
		if !environmentName.MatchString(value) {
			return errors.New("pushover " + key + " must be an environment variable name")
		}
		if key == "token_env" {
			p.TokenEnv = value
		} else {
			p.UserEnv = value
		}
	case "priority":
		if _, ok := PushoverPriorities[value]; !ok {
			return errors.New("pushover priority must be lowest, low, normal, high, or emergency")
		}
		p.Priority = value
	case "retry":
		if d, err := time.ParseDuration(value); err != nil || d < 30*time.Second || d%time.Second != 0 {
			return errors.New("pushover retry must be whole seconds of at least 30s, such as 2m")
		}
		p.Retry = value
	case "expire":
		if d, err := time.ParseDuration(value); err != nil || d <= 0 || d > 3*time.Hour || d%time.Second != 0 {
			return errors.New("pushover expire must be whole seconds up to 3h, such as 1h")
		}
		p.Expire = value
	case "device":
		if !pushoverDevice.MatchString(value) {
			return errors.New("pushover device must be device names (letters, digits, _ or -), separated by commas")
		}
		p.Device = value
	default:
		return errors.New("unknown pushover option " + key)
	}
	return nil
}

// Emergency reports whether the step asks Pushover to repeat until acknowledged.
func (p Pushover) Emergency() bool { return p.Priority == "emergency" }

func (p Pushover) merge(base Pushover) Pushover {
	if p.TokenEnv == "" {
		p.TokenEnv = base.TokenEnv
	}
	if p.UserEnv == "" {
		p.UserEnv = base.UserEnv
	}
	if p.Priority == "" {
		p.Priority = base.Priority
	}
	if p.Retry == "" {
		p.Retry = base.Retry
	}
	if p.Expire == "" {
		p.Expire = base.Expire
	}
	if p.Device == "" {
		p.Device = base.Device
	}
	return p
}

func (p Pushover) validate() error {
	if p.TokenEnv == "" || p.UserEnv == "" {
		return errors.New("pushover.token_env and pushover.user_env (or step options) are required for a pushover step")
	}
	if p.Emergency() && (p.Retry == "" || p.Expire == "") {
		return errors.New("pushover priority emergency requires retry and expire")
	}
	return nil
}

// PushoverSettings returns the settings receipt commands and bare inspection
// use: the first effective emergency pushover step (its token issued the
// receipts), else the first pushover step, else the top-level pushover block.
func PushoverSettings(cfg Config) Pushover {
	steps, _ := EffectiveNotifySteps(cfg)
	first := cfg.Pushover
	found := false
	for _, step := range steps {
		if step.Via != "pushover" {
			continue
		}
		if step.Emergency() {
			return step.Pushover
		}
		if !found {
			first, found = step.Pushover, true
		}
	}
	return first
}
