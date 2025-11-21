package server

import (
	"context"
	"fmt"
	"net"
	"sync"

	"google.golang.org/grpc"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/guilhem/operator-plugin-framework/registry"
)

// Server manages bidirectional plugin connections via gRPC.
// Authentication is delegated to kube-rbac-proxy sidecar.
// Plugins are automatically registered when they connect via the PluginStream RPC.
type Server struct {
	addr           string
	maxConnections int
	registry       *registry.Manager
	streamManager  *StreamManager
	mu             sync.RWMutex
	grpcServer     *grpc.Server
	listener       net.Listener
	isRunning      bool
}

// New creates a new plugin server.
// Authentication is handled by kube-rbac-proxy sidecar.
// Plugins are automatically registered on connection via HandlePluginStream.
func New(addr string, opts ...ServerOption) *Server {
	s := &Server{
		addr:           addr,
		maxConnections: 100,
		registry:       registry.New(),
	}

	for _, opt := range opts {
		opt(s)
	}

	// Create stream manager for automatic plugin registration
	s.streamManager = NewStreamManager(s)

	return s
}

// GetRegistry returns the plugin registry for plugin management
func (s *Server) GetRegistry() *registry.Manager {
	return s.registry
}

// ListPlugins returns a list of currently connected plugin names
func (s *Server) ListPlugins() []string {
	return s.streamManager.ListConnectedPlugins()
}

// IsPluginConnected checks if a plugin is currently connected
func (s *Server) IsPluginConnected(name string) bool {
	return s.streamManager.IsPluginConnected(name)
}

// GetStreamManager returns the StreamManager for handling plugin connections.
// This is used to call HandlePluginStream from your gRPC service implementation.
func (s *Server) GetStreamManager() *StreamManager {
	return s.streamManager
}

// HandlePluginStream is a convenience method that delegates to the StreamManager.
// Call this from your gRPC PluginStream RPC implementation.
// The plugin will be automatically registered on successful connection.
func (s *Server) HandlePluginStream(ctx context.Context, pluginName string) error {
	return s.streamManager.HandlePluginStream(ctx, pluginName)
}

// Start starts the plugin server and listens for plugin connections.
// It blocks until ctx is cancelled, then gracefully shuts down.
// Implements controller-runtime Runnable interface: Start(ctx context.Context) error
func (s *Server) Start(ctx context.Context) error {
	logger := log.FromContext(ctx)

	// Parse address
	network, addr, err := parseAddr(s.addr)
	if err != nil {
		return fmt.Errorf("invalid server address: %w", err)
	}

	// Create listener
	lis, err := net.Listen(network, addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s %s: %w", network, addr, err)
	}
	s.listener = lis

	// Create gRPC server
	s.grpcServer = grpc.NewServer()

	logger.Info("Starting plugin server", "network", network, "addr", addr)

	// Mark server as running
	s.mu.Lock()
	s.isRunning = true
	s.mu.Unlock()

	// Start gRPC server in goroutine
	errs := make(chan error, 1)
	go func() {
		if err := s.grpcServer.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			logger.Error(err, "gRPC server error")
			errs <- err
		}
	}()

	// Block on context cancellation
	select {
	case <-ctx.Done():
		logger.Info("Context cancelled, shutting down plugin server")
		s.Stop()
		return ctx.Err()
	case err := <-errs:
		s.Stop()
		return err
	}
}

// Stop gracefully stops the plugin server
func (s *Server) Stop() {
	logger := log.Log
	logger.Info("Stopping plugin server")

	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isRunning {
		return
	}

	// Gracefully stop gRPC server
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
		s.grpcServer = nil
	}

	// Close listener
	if s.listener != nil {
		_ = s.listener.Close()
		s.listener = nil
	}

	s.isRunning = false
}

// IsRunning returns whether the server is currently running
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.isRunning
}

// ConnectionCount returns the number of active plugin connections
func (s *Server) ConnectionCount() int {
	return s.streamManager.ConnectionCount()
}

// parseAddr parses an address string into network and address components.
// Supported schemes: "unix:///path/to/socket" and "tcp://host:port"
func parseAddr(addr string) (string, string, error) {
	if len(addr) < 8 {
		return "", "", fmt.Errorf("invalid address format: %s", addr)
	}

	scheme := addr[:5]
	switch scheme {
	case "unix:":
		// unix:///path/to/socket -> network="unix", addr="/path/to/socket"
		return "unix", addr[7:], nil
	case "tcp:/":
		// tcp://host:port -> network="tcp", addr="host:port"
		return "tcp", addr[6:], nil
	default:
		return "", "", fmt.Errorf("unsupported address scheme: %s", scheme)
	}
}
