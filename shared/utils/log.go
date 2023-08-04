package utils

import (
	"context"

	"github.com/quintans/eventsourcing/log"
)

var logKey = struct{}{}

func LogToCtx(ctx context.Context, logger log.Logger) context.Context {
	return context.WithValue(ctx, logKey, logger)
}

func LogFromCtx(ctx context.Context) log.Logger {
	l := ctx.Value(logKey)
	return l.(log.Logger)
}
