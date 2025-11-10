package sources

import (
	"context"

	"github.com/aslakknutsen/kkbase/pkg/agenttypes"
)

// EventSource is the interface for all event sources
type EventSource interface {
	// Name returns the name of the event source
	Name() string

	// Start starts the event source
	Start(ctx context.Context) error

	// Events returns the channel for receiving events
	Events() <-chan agenttypes.Event

	// Stop stops the event source
	Stop() error
}
