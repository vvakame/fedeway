package log

import (
	"context"

	"github.com/go-logr/logr"
)

func FromContext(ctx context.Context) logr.Logger {
	return logr.FromContextOrDiscard(ctx)
}

func WithLogger(ctx context.Context, logger logr.Logger) context.Context {
	return logr.NewContext(ctx, logger)
}
