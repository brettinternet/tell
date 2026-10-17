package delivery

import (
	"context"
	"github.com/brettinternet/tell/internal/config"
	"os"
)

type childConfigKey struct{}

func childContext(ctx context.Context, cfg config.Config) context.Context {
	return context.WithValue(ctx, childConfigKey{}, cfg)
}
func childEnv(ctx context.Context, env []string) []string {
	if env == nil {
		env = os.Environ()
	}
	if cfg, ok := ctx.Value(childConfigKey{}).(config.Config); ok {
		return cfg.ChildEnv(env)
	}
	return env
}
