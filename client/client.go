package client

import (
	"context"
	"fmt"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TokenProvider is an interface for providing authentication tokens
type TokenProvider interface {
	// GetToken returns the authentication token
	GetToken() (string, error)
}

// ServiceAccountTokenProvider reads token from Kubernetes ServiceAccount mount
type ServiceAccountTokenProvider struct {
	tokenPath string
}

// NewServiceAccountTokenProvider creates a provider that reads from the default ServiceAccount token path
func NewServiceAccountTokenProvider() *ServiceAccountTokenProvider {
	return &ServiceAccountTokenProvider{
		tokenPath: "/var/run/secrets/kubernetes.io/serviceaccount/token",
	}
}

// NewServiceAccountTokenProviderWithPath creates a provider with a custom token path
func NewServiceAccountTokenProviderWithPath(path string) *ServiceAccountTokenProvider {
	return &ServiceAccountTokenProvider{
		tokenPath: path,
	}
}

// GetToken reads and returns the ServiceAccount token
func (p *ServiceAccountTokenProvider) GetToken() (string, error) {
	tokenBytes, err := os.ReadFile(p.tokenPath)
	if err != nil {
		return "", fmt.Errorf("failed to read ServiceAccount token: %w", err)
	}
	return string(tokenBytes), nil
}

// StaticTokenProvider provides a static token (for testing or custom scenarios)
type StaticTokenProvider struct {
	token string
}

// NewStaticTokenProvider creates a provider with a static token
func NewStaticTokenProvider(token string) *StaticTokenProvider {
	return &StaticTokenProvider{token: token}
}

// GetToken returns the static token
func (p *StaticTokenProvider) GetToken() (string, error) {
	return p.token, nil
}

// tokenAuth implements credentials.PerRPCCredentials for bearer token authentication
type tokenAuth struct {
	token string
}

func (t *tokenAuth) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		"authorization": "Bearer " + t.token,
	}, nil
}

func (t *tokenAuth) RequireTransportSecurity() bool {
	return false // kube-rbac-proxy handles TLS
}

// Connection represents a plugin connection to the operator server
type Connection struct {
	addr          string
	name          string
	tokenProvider TokenProvider
	conn          *grpc.ClientConn
}

// ClientOption is a functional option for Connection configuration
type ClientOption func(*Connection)

// WithTokenProvider sets a custom token provider for authentication
func WithTokenProvider(provider TokenProvider) ClientOption {
	return func(c *Connection) {
		c.tokenProvider = provider
	}
}

// WithServiceAccountToken enables ServiceAccount token authentication (default path)
func WithServiceAccountToken() ClientOption {
	return func(c *Connection) {
		c.tokenProvider = NewServiceAccountTokenProvider()
	}
}

// WithServiceAccountTokenPath enables ServiceAccount token authentication with custom path
func WithServiceAccountTokenPath(path string) ClientOption {
	return func(c *Connection) {
		c.tokenProvider = NewServiceAccountTokenProviderWithPath(path)
	}
}

// WithStaticToken sets a static token for authentication (for testing)
func WithStaticToken(token string) ClientOption {
	return func(c *Connection) {
		c.tokenProvider = NewStaticTokenProvider(token)
	}
}

// New creates a new plugin connection.
// addr should be in format "unix:///path/to/socket" or "https://host:port" (for kube-rbac-proxy)
//
// If a TokenProvider is configured via WithTokenProvider/WithServiceAccountToken,
// the token will be sent as "Authorization: Bearer <token>" header in all gRPC requests.
//
// Note: ctx is currently not used by grpc.NewClient() but kept in signature
// for future use (connection timeouts, context propagation, etc.)
func New(ctx context.Context, name string, addr string, opts ...ClientOption) (*Connection, error) {
	c := &Connection{
		addr: addr,
		name: name,
	}

	for _, opt := range opts {
		opt(c)
	}

	// Prepare gRPC dial options
	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	// Add token authentication if provider is configured
	if c.tokenProvider != nil {
		token, err := c.tokenProvider.GetToken()
		if err != nil {
			return nil, fmt.Errorf("failed to get authentication token: %w", err)
		}
		dialOpts = append(dialOpts, grpc.WithPerRPCCredentials(&tokenAuth{token: token}))
	}

	// Create gRPC connection
	conn, err := grpc.NewClient(addr, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
	}

	c.conn = conn
	return c, nil
}

// Close closes the plugin connection
func (c *Connection) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Name returns the plugin name
func (c *Connection) Name() string {
	return c.name
}

// GetConnection returns the underlying gRPC connection
func (c *Connection) GetConnection() *grpc.ClientConn {
	return c.conn
}
