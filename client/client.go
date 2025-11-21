// Package client provides gRPC client functionality for connecting plugins to operators
// with secure authentication through kube-rbac-proxy.
//
// # Authentication Architecture
//
// The client supports flexible token-based authentication through a provider pattern:
//
//   - TokenProvider: Interface for obtaining authentication tokens
//   - TokenCredential: gRPC credential implementation that uses a TokenProvider
//   - Client options configure which TokenProvider to use
//
// # Supported Authentication Methods
//
// 1. ServiceAccount tokens (Kubernetes native):
//   - WithServiceAccountToken(): Uses default ServiceAccount token path
//   - WithServiceAccountTokenPath(path): Uses custom token file path
//
// 2. Static tokens (for testing/development):
//   - WithStaticToken(token): Uses a fixed token string
//
// 3. Custom providers:
//   - WithTokenProvider(provider): Uses any implementation of TokenProvider
//
// # Example Usage
//
//	conn, err := client.New(
//	    ctx,
//	    "my-plugin",
//	    "https://operator:8443",
//	    "v1.0.0",
//	    &pb.MyService_ServiceDesc,
//	    &myServiceImpl{},
//	    streamCreator,
//	    client.WithServiceAccountToken(), // Uses ServiceAccount authentication
//	)
//	if err != nil {
//	    return err
//	}
//	defer conn.Close()
//
// The client automatically handles:
// - Token retrieval from the configured provider
// - Bearer token authentication in gRPC requests
// - Connection lifecycle management
package client

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/guilhem/operator-plugin-framework/stream"
	"github.com/guilhem/operator-plugin-framework/token"
)

// connectionConfig holds the configuration for creating a connection
type connectionConfig struct {
	addr          string
	name          string
	tokenProvider token.TokenProvider
}

// ClientOption is a functional option for connection configuration
type ClientOption func(*connectionConfig)

// WithTokenProvider sets a custom token provider for authentication
func WithTokenProvider(provider token.TokenProvider) ClientOption {
	return func(c *connectionConfig) {
		c.tokenProvider = provider
	}
}

// WithServiceAccountToken enables ServiceAccount token authentication using the default Kubernetes path.
// This configures a ServiceAccountTokenProvider that reads from /var/run/secrets/kubernetes.io/serviceaccount/token.
func WithServiceAccountToken() ClientOption {
	return func(c *connectionConfig) {
		c.tokenProvider = token.NewServiceAccountTokenProvider()
	}
}

// WithServiceAccountTokenPath enables ServiceAccount token authentication with a custom file path.
// This is useful for testing or non-standard Kubernetes environments.
func WithServiceAccountTokenPath(path string) ClientOption {
	return func(c *connectionConfig) {
		c.tokenProvider = token.NewServiceAccountTokenProviderWithPath(path)
	}
}

// WithStaticToken sets a static token for authentication (primarily for testing).
// The token is used as-is for Bearer authentication in all gRPC requests.
func WithStaticToken(tokenS string) ClientOption {
	return func(c *connectionConfig) {
		c.tokenProvider = token.NewStaticTokenProvider(tokenS)
	}
}

type StreamCreatorFunc func(conn *grpc.ClientConn) (stream.StreamInterface, error)

type Client struct {
	stream.PluginStreamClient

	conn *grpc.ClientConn
}

// New creates a new plugin stream client with authentication and stream setup.
// This is a convenience function that handles the complete plugin connection process.
//
// The function supports flexible authentication through TokenProvider options:
// - WithServiceAccountToken(): Uses Kubernetes ServiceAccount tokens
// - WithStaticToken(token): Uses a fixed token (for testing)
// - WithTokenProvider(provider): Uses a custom token provider implementation
//
// When a token provider is configured, the client automatically:
// 1. Validates the provider by attempting to retrieve a token immediately
// 2. Retrieves fresh tokens from the provider for each gRPC request
// 3. Adds Bearer authentication headers to all gRPC calls
// 4. Handles token refresh through the provider interface
//
// Parameters:
//   - ctx: context for the connection (used for cancellation)
//   - name: plugin name (used for registration)
//   - addr: server address (unix:///path/to/socket or https://host:port)
//   - pluginVersion: plugin version string (sent during registration)
//   - serviceDesc: gRPC service descriptor for RPC routing
//   - impl: service implementation (handles incoming RPC calls)
//   - streamCreator: function that creates the bidirectional stream
//   - opts: client options for authentication and configuration
//
// Returns a PluginStreamClient ready to handle RPC calls, or an error if connection fails.
func New(
	ctx context.Context,
	name string,
	addr string,
	pluginVersion string,
	serviceDesc grpc.ServiceDesc,
	impl any,
	streamCreator StreamCreatorFunc,
	opts ...ClientOption,
) (*Client, error) {
	// Create connection config
	conn := &connectionConfig{
		addr: addr,
		name: name,
	}

	// Apply options
	for _, opt := range opts {
		opt(conn)
	}

	// Prepare gRPC dial options
	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	// Add token authentication if provider is configured
	if conn.tokenProvider != nil {
		// Validate the token provider by attempting to get a token
		_, err := conn.tokenProvider.GetToken()
		if err != nil {
			return nil, fmt.Errorf("failed to validate token provider: %w", err)
		}
		dialOpts = append(dialOpts, grpc.WithPerRPCCredentials(&token.TokenCredential{Provider: conn.tokenProvider}))
	}

	// Create gRPC connection
	grpcConn, err := grpc.NewClient(addr, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
	}
	defer func() {
		if err != nil {
			_ = grpcConn.Close()
		}
	}()

	// Create the stream
	grpcStream, err := streamCreator(grpcConn)
	if err != nil {
		return nil, fmt.Errorf("failed to create plugin stream: %w", err)
	}

	// Create and return the plugin stream client
	pluginStreamClient, err := stream.NewPluginStreamClient(ctx, grpcStream, name, pluginVersion, serviceDesc, impl)
	if err != nil {
		return nil, fmt.Errorf("failed to create plugin stream client: %w", err)
	}

	return &Client{
		PluginStreamClient: *pluginStreamClient,
		conn:               grpcConn,
	}, nil
}

// Close closes the underlying gRPC connection.
// This should be called when the client is shutting down to ensure proper cleanup.
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
