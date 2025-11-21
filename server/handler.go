package server

import "context"

// StreamMessage is a generic interface for any message type that can be sent/received on the stream.
// Implementations must support marshaling/unmarshaling.
type StreamMessage interface {
	// Validate checks if the message is valid
	Validate() error
}

// PluginMessageHandler is a generic handler for processing plugin messages.
// T is the type of message the handler processes.
// The handler receives messages from a plugin stream and can return responses
// or errors to be sent back to the plugin.
type PluginMessageHandler[T StreamMessage] interface {
	// HandlePluginMessage processes an incoming message from a plugin.
	// ctx: the context for the request (can be cancelled by plugin stream)
	// pluginName: the name of the plugin sending the message
	// msg: the message to process
	// Returns: response message, error if any, or (nil, nil) if no response needed
	HandlePluginMessage(ctx context.Context, pluginName string, msg T) (T, error)
}

// PluginConnectionHandler handles lifecycle events of plugin connections.
// This is optional and can be used for logging, metrics, cleanup, etc.
type PluginConnectionHandler interface {
	// OnPluginConnect is called when a plugin successfully connects and registers.
	// This happens after the PluginRegister message is received and validated.
	OnPluginConnect(pluginName string) error

	// OnPluginDisconnect is called when a plugin disconnects.
	// This can happen due to stream closure, context cancellation, or errors.
	OnPluginDisconnect(pluginName string) error
}

// NoOpPluginConnectionHandler is a default implementation that does nothing.
// Use this if you don't need connection lifecycle tracking.
type NoOpPluginConnectionHandler struct{}

func (n *NoOpPluginConnectionHandler) OnPluginConnect(pluginName string) error {
	return nil
}

func (n *NoOpPluginConnectionHandler) OnPluginDisconnect(pluginName string) error {
	return nil
}
