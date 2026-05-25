package compact

import (
	"context"

	"github.com/ctimbi/test/internal/api"
)

// NoCompaction is the default — never modifies messages.
type NoCompaction struct{}

func (NoCompaction) Compact(_ context.Context, messages []api.Message) ([]api.Message, error) {
	return messages, nil
}
