package client

import (
	"context"
	"fmt"
	"io"
)

// PluginStreamAdapter adapts a plugin's message handler to work with gRPC bidirectional streams.
// This simplifies plugin implementations by handling the stream lifecycle automatically.
type PluginStreamAdapter[Msg any] struct {
	stream  BidiStream[Msg]
	handler func(ctx context.Context, msg Msg) (Msg, error)
}

// BidiStream represents a bidirectional gRPC stream with Send/Recv.
type BidiStream[Msg any] interface {
	Send(msg Msg) error
	Recv() (Msg, error)
}

// NewPluginStreamAdapter creates a new adapter for plugin stream handling.
// handler: processes incoming messages and returns responses
// stream: the bidirectional gRPC stream
func NewPluginStreamAdapter[Msg any](
	stream BidiStream[Msg],
	handler func(ctx context.Context, msg Msg) (Msg, error),
) *PluginStreamAdapter[Msg] {
	return &PluginStreamAdapter[Msg]{
		stream:  stream,
		handler: handler,
	}
}

// Run starts the message loop until context cancellation or stream closure.
// It blocks and returns only when the stream ends or an error occurs.
func (psa *PluginStreamAdapter[Msg]) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Receive message
		msg, err := psa.stream.Recv()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("receive error: %w", err)
		}

		// Handle message
		response, err := psa.handler(ctx, msg)
		if err != nil {
			// Could optionally send error response here
			return fmt.Errorf("handler error: %w", err)
		}

		// Send response
		if err := psa.stream.Send(response); err != nil {
			return fmt.Errorf("send error: %w", err)
		}
	}
}
