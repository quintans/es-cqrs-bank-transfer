package app

import "context"

type Tx func(ctx context.Context, f func(ctx context.Context) error) error
