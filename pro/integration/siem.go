package integration

import "context"

type SIEMClient interface {
	Export(ctx context.Context, events []any) error
}
