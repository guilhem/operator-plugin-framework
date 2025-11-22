package server

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"sigs.k8s.io/controller-runtime/pkg/log"

	pluginframeworkv1 "github.com/guilhem/operator-plugin-framework/pluginframework/v1"
)

// PluginFrameworkServiceServerImpl implements the generated PluginFrameworkServiceServer.
// It integrates with the existing Server and StreamManager to provide a complete
// plugin communication framework.
type PluginFrameworkServiceServerImpl struct {
	pluginframeworkv1.UnimplementedPluginFrameworkServiceServer
	server *Server
}

// NewPluginFrameworkServiceServer creates a new service server implementation.
func NewPluginFrameworkServiceServer(server *Server) *PluginFrameworkServiceServerImpl {
	return &PluginFrameworkServiceServerImpl{
		server: server,
	}
}

// PluginStream implements the bidirectional streaming RPC for plugin communication.
// This method handles the complete plugin lifecycle:
// 1. Receives PluginRegister message
// 2. Registers the plugin automatically
// 3. Manages the bidirectional stream for RPC calls and responses
func (s *PluginFrameworkServiceServerImpl) PluginStream(stream grpc.BidiStreamingServer[pluginframeworkv1.PluginStreamMessage, pluginframeworkv1.PluginStreamMessage]) error {
	logger := log.FromContext(stream.Context())

	// Step 1: Wait for plugin registration
	msg, err := stream.Recv()
	if err != nil {
		logger.Error(err, "Failed to receive initial message from plugin")
		return status.Errorf(codes.InvalidArgument, "failed to receive plugin registration: %v", err)
	}

	// Validate that first message is PluginRegister
	register := msg.GetRegister()
	if register == nil {
		logger.Error(nil, "First message must be PluginRegister", "payload", msg.GetPayload())
		return status.Errorf(codes.InvalidArgument, "first message must be PluginRegister")
	}

	pluginName := register.GetName()
	if pluginName == "" {
		return status.Errorf(codes.InvalidArgument, "plugin name cannot be empty")
	}

	logger.Info("Plugin attempting to connect", "plugin", pluginName, "version", register.GetVersion())

	// Step 2: Use the existing StreamManager to handle the connection
	// This automatically registers the plugin and manages the connection lifecycle
	err = s.server.HandlePluginStream(stream.Context(), pluginName)
	if err != nil {
		logger.Error(err, "Failed to handle plugin stream", "plugin", pluginName)
		return status.Errorf(codes.Internal, "failed to handle plugin stream: %v", err)
	}

	// Step 3: Keep the stream alive for bidirectional communication
	// The StreamManager handles the lifecycle, so we just need to keep the gRPC stream open
	// In a real implementation, you would integrate with the stream package's StreamManager
	// to handle RPC calls and responses

	logger.Info("Plugin stream established", "plugin", pluginName)

	// For now, we keep the stream open until context cancellation
	// Future enhancement: integrate with stream.StreamManager for full RPC support
	<-stream.Context().Done()

	logger.Info("Plugin stream closed", "plugin", pluginName)
	return stream.Context().Err()
}
