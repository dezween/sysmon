package port

import "context"

// Keeper runs the activity-keeping loop until ctx is done.
type Keeper interface {
	Run(ctx context.Context) error
}
