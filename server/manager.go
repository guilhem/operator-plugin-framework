package server

import (
	"context"
	"sync"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

// StreamManager manages bidirectional plugin streams and handles automatic plugin registration.
// It simplifies the implementation by:
// 1. Reading the PluginRegister message first
// 2. Automatically registering the plugin
// 3. Calling the connection handler
// 4. Managing the stream lifetime
type StreamManager struct {
	server            *Server
	connectionHandler PluginConnectionHandler
	streamTimeout     time.Duration
	maxMessageSize    int
	mu                sync.Mutex
	activeStreams     map[string]*ManagedStream
}

// ManagedStream represents a managed plugin stream with automatic registration.
type ManagedStream struct {
	pluginName  string
	createdAt   time.Time
	lastMessage time.Time
	closeCh     chan struct{}
	mu          sync.Mutex
}

// NewStreamManager creates a new stream manager for the server.
func NewStreamManager(server *Server, opts ...StreamManagerOption) *StreamManager {
	sm := &StreamManager{
		server:            server,
		connectionHandler: &NoOpPluginConnectionHandler{},
		streamTimeout:     5 * time.Minute,
		maxMessageSize:    10 * 1024 * 1024, // 10MB default
		activeStreams:     make(map[string]*ManagedStream),
	}

	for _, opt := range opts {
		opt(sm)
	}

	return sm
}

// StreamManagerOption is a functional option for StreamManager configuration.
type StreamManagerOption func(*StreamManager)

// WithConnectionHandler sets a custom connection handler for plugin lifecycle events.
func WithConnectionHandler(handler PluginConnectionHandler) StreamManagerOption {
	return func(sm *StreamManager) {
		if handler != nil {
			sm.connectionHandler = handler
		}
	}
}

// WithStreamIdleTimeout sets the timeout for stream operations.
func WithStreamIdleTimeout(timeout time.Duration) StreamManagerOption {
	return func(sm *StreamManager) {
		if timeout > 0 {
			sm.streamTimeout = timeout
		}
	}
}

// WithMaxStreamMessageSize sets the maximum size for stream messages.
func WithMaxStreamMessageSize(size int) StreamManagerOption {
	return func(sm *StreamManager) {
		if size > 0 {
			sm.maxMessageSize = size
		}
	}
}

// HandlePluginStream manages a new plugin stream connection.
// This function:
// 1. Waits for the first PluginRegister message
// 2. Registers the plugin automatically
// 3. Calls the connection handler
// 4. Keeps the stream alive until closure or error
//
// This is meant to be called from the gRPC service implementation.
func (sm *StreamManager) HandlePluginStream(ctx context.Context, pluginName string) error {
	logger := log.FromContext(ctx)

	// Check connection limit before registering
	sm.mu.Lock()
	if len(sm.activeStreams) >= sm.server.maxConnections {
		sm.mu.Unlock()
		logger.Info("Max connections reached, rejecting new plugin", "plugin", pluginName)
		return ErrMaxConnectionsReached
	}
	sm.mu.Unlock()

	// Create managed stream
	ms := &ManagedStream{
		pluginName:  pluginName,
		createdAt:   time.Now(),
		lastMessage: time.Now(),
		closeCh:     make(chan struct{}),
	}

	// Register the plugin
	sm.mu.Lock()
	sm.activeStreams[pluginName] = ms
	sm.mu.Unlock()

	logger.Info("Plugin registered", "plugin", pluginName)

	// Call connection handler
	if err := sm.connectionHandler.OnPluginConnect(pluginName); err != nil {
		logger.Error(err, "Connection handler failed", "plugin", pluginName)
		sm.unregisterPlugin(pluginName)
		return err
	}

	// Keep stream alive until context cancellation or error
	<-ctx.Done()

	// Disconnect handler
	sm.unregisterPlugin(pluginName)
	_ = sm.connectionHandler.OnPluginDisconnect(pluginName)

	return ctx.Err()
}

// unregisterPlugin safely removes a plugin from tracking.
func (sm *StreamManager) unregisterPlugin(pluginName string) {
	logger := log.Log

	// Remove from active streams
	sm.mu.Lock()
	if ms, exists := sm.activeStreams[pluginName]; exists {
		close(ms.closeCh)
		delete(sm.activeStreams, pluginName)
	}
	sm.mu.Unlock()

	logger.Info("Plugin unregistered", "plugin", pluginName)
}

// IsPluginConnected checks if a plugin has an active managed stream.
func (sm *StreamManager) IsPluginConnected(pluginName string) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	_, exists := sm.activeStreams[pluginName]
	return exists
}

// GetPluginStream returns the managed stream for a plugin, or nil if not connected.
func (sm *StreamManager) GetPluginStream(pluginName string) *ManagedStream {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	return sm.activeStreams[pluginName]
}

// ListConnectedPlugins returns a list of currently connected plugin names.
func (sm *StreamManager) ListConnectedPlugins() []string {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	names := make([]string, 0, len(sm.activeStreams))
	for name := range sm.activeStreams {
		names = append(names, name)
	}
	return names
}

// ConnectionCount returns the number of active plugin streams.
func (sm *StreamManager) ConnectionCount() int {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	return len(sm.activeStreams)
}

// UpdateLastMessageTime updates the last message timestamp for a plugin.
// This can be used for timeout/health tracking.
func (sm *StreamManager) UpdateLastMessageTime(pluginName string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if ms, exists := sm.activeStreams[pluginName]; exists {
		ms.mu.Lock()
		ms.lastMessage = time.Now()
		ms.mu.Unlock()
	}
}

// GetPluginInfo returns connection info for a plugin.
func (sm *StreamManager) GetPluginInfo(pluginName string) *PluginStreamInfo {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	ms, exists := sm.activeStreams[pluginName]
	if !exists {
		return nil
	}

	ms.mu.Lock()
	defer ms.mu.Unlock()

	return &PluginStreamInfo{
		Name:          ms.pluginName,
		ConnectedAt:   ms.createdAt,
		LastMessageAt: ms.lastMessage,
		Uptime:        time.Since(ms.createdAt),
	}
}

// PluginStreamInfo contains information about a plugin's stream connection.
type PluginStreamInfo struct {
	Name          string
	ConnectedAt   time.Time
	LastMessageAt time.Time
	Uptime        time.Duration
}
