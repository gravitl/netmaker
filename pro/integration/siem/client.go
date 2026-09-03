package siem

import "context"

type Client interface {
	Export(ctx context.Context, events []any) error
}
