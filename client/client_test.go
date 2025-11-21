package client

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"

	pluginframeworkv1 "github.com/guilhem/operator-plugin-framework/pluginframework/v1"
	"github.com/guilhem/operator-plugin-framework/stream"
	"github.com/guilhem/operator-plugin-framework/token"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		opts    []ClientOption
		wantErr bool
	}{
		{
			name:    "unix socket - connection setup succeeds",
			addr:    "unix:///tmp/test.sock",
			wantErr: false, // New succeeds even if connection fails later
		},
		{
			name:    "tcp address - connection setup succeeds",
			addr:    "tcp://localhost:50051",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
			defer cancel()

			// Mock service descriptor and implementation for testing
			mockServiceDesc := grpc.ServiceDesc{
				ServiceName: "test.TestService",
				Methods:     []grpc.MethodDesc{},
				Streams:     []grpc.StreamDesc{},
			}

			var mockImpl interface{} = &mockServiceImpl{}

			_, err := New(ctx, "test-plugin", tt.addr, "v1.0.0", mockServiceDesc, mockImpl, mockStreamCreator, tt.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

// mockServiceImpl is a mock implementation for testing
type mockServiceImpl struct{}

// mockStream is a simple mock implementation of StreamInterface for testing
type mockStream struct {
	ctx context.Context
}

func (m *mockStream) Send(msg *pluginframeworkv1.PluginStreamMessage) error {
	// Accept registration messages
	if register := msg.GetRegister(); register != nil {
		return nil
	}
	return fmt.Errorf("unexpected message type")
}

func (m *mockStream) Recv() (*pluginframeworkv1.PluginStreamMessage, error) {
	// For testing, just return an error to indicate end of stream
	return nil, fmt.Errorf("mock stream closed")
}

func (m *mockStream) Context() context.Context {
	return m.ctx
}

// mockStreamCreator is a mock stream creator for testing
func mockStreamCreator(conn *grpc.ClientConn) (stream.StreamInterface, error) {
	return &mockStream{ctx: context.Background()}, nil
}

func TestServiceAccountTokenProvider(t *testing.T) {
	// Create temporary token file
	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, "token")
	expectedToken := "test-sa-token-12345"

	if err := os.WriteFile(tokenPath, []byte(expectedToken), 0600); err != nil {
		t.Fatalf("failed to create test token file: %v", err)
	}

	// Test with custom path
	provider := token.NewServiceAccountTokenProviderWithPath(tokenPath)
	token, err := provider.GetToken()
	if err != nil {
		t.Fatalf("GetToken() error = %v", err)
	}
	if token != expectedToken {
		t.Errorf("GetToken() = %q, want %q", token, expectedToken)
	}
}

func TestServiceAccountTokenProvider_FileNotFound(t *testing.T) {
	provider := token.NewServiceAccountTokenProviderWithPath("/nonexistent/token")
	_, err := provider.GetToken()
	if err == nil {
		t.Error("GetToken() expected error for nonexistent file, got nil")
	}
}

func TestStaticTokenProvider(t *testing.T) {
	expectedToken := "static-token-xyz"
	provider := token.NewStaticTokenProvider(expectedToken)

	token, err := provider.GetToken()
	if err != nil {
		t.Fatalf("GetToken() error = %v", err)
	}
	if token != expectedToken {
		t.Errorf("GetToken() = %q, want %q", token, expectedToken)
	}
}

func TestWithTokenProvider(t *testing.T) {
	provider := token.NewStaticTokenProvider("test-token")

	config := &connectionConfig{}
	opt := WithTokenProvider(provider)
	opt(config)

	if config.tokenProvider != provider {
		t.Error("WithTokenProvider() did not set token provider")
	}
}

func TestWithServiceAccountToken(t *testing.T) {
	config := &connectionConfig{}
	opt := WithServiceAccountToken()
	opt(config)

	if config.tokenProvider == nil {
		t.Error("WithServiceAccountToken() did not set token provider")
	}

	// Verify it's a ServiceAccountTokenProvider
	_, ok := config.tokenProvider.(*token.ServiceAccountTokenProvider)
	if !ok {
		t.Errorf("WithServiceAccountToken() set wrong provider type: %T", config.tokenProvider)
	}
}

func TestWithServiceAccountTokenPath(t *testing.T) {
	customPath := "/custom/path/token"
	config := &connectionConfig{}
	opt := WithServiceAccountTokenPath(customPath)
	opt(config)

	if config.tokenProvider == nil {
		t.Error("WithServiceAccountTokenPath() did not set token provider")
	}

	// Verify it's a ServiceAccountTokenProvider
	_, ok := config.tokenProvider.(*token.ServiceAccountTokenProvider)
	if !ok {
		t.Errorf("WithServiceAccountTokenPath() set wrong provider type: %T", config.tokenProvider)
	}
}

func TestWithStaticToken(t *testing.T) {
	expectedToken := "my-static-token"
	config := &connectionConfig{}
	opt := WithStaticToken(expectedToken)
	opt(config)

	if config.tokenProvider == nil {
		t.Error("WithStaticToken() did not set token provider")
	}

	token, err := config.tokenProvider.GetToken()
	if err != nil {
		t.Fatalf("GetToken() error = %v", err)
	}
	if token != expectedToken {
		t.Errorf("GetToken() = %q, want %q", token, expectedToken)
	}
}

func TestTokenCredential_GetRequestMetadata(t *testing.T) {
	provider := token.NewStaticTokenProvider("test-token-123")
	cred := &token.TokenCredential{Provider: provider}

	metadata, err := cred.GetRequestMetadata(t.Context())
	if err != nil {
		t.Fatalf("GetRequestMetadata() error = %v", err)
	}

	expectedAuth := "Bearer test-token-123"
	if metadata["authorization"] != expectedAuth {
		t.Errorf("authorization header = %q, want %q", metadata["authorization"], expectedAuth)
	}
}

func TestTokenCredential_RequireTransportSecurity(t *testing.T) {
	provider := token.NewStaticTokenProvider("test")
	cred := &token.TokenCredential{Provider: provider}

	// Should return false since kube-rbac-proxy handles TLS
	if cred.RequireTransportSecurity() {
		t.Error("RequireTransportSecurity() = true, want false")
	}
}

func TestConnectionName(t *testing.T) {
	// This test is no longer relevant since Connection struct was removed
	t.Skip("Connection struct removed - test no longer applicable")
}

func TestConnectionWithToken(t *testing.T) {
	// This test is no longer relevant since Connection struct was removed
	t.Skip("Connection struct removed - test no longer applicable")
}

func TestNew_WithInvalidToken(t *testing.T) {
	// Provider that returns an error
	provider := token.NewServiceAccountTokenProviderWithPath("/nonexistent/token")

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	// Mock service descriptor and implementation
	mockServiceDesc := grpc.ServiceDesc{
		ServiceName: "test.TestService",
		Methods:     []grpc.MethodDesc{},
		Streams:     []grpc.StreamDesc{},
	}
	var mockImpl interface{} = &mockServiceImpl{}

	_, err := New(ctx, "test-plugin", "unix:///tmp/test.sock", "v1.0.0", mockServiceDesc, mockImpl, mockStreamCreator,
		WithTokenProvider(provider),
	)

	if err == nil {
		t.Error("New() expected error when token provider fails, got nil")
	}
}

// failingMockStream is a mock stream that fails on Send for registration messages
type failingMockStream struct {
	ctx context.Context
}

func (m *failingMockStream) Send(msg *pluginframeworkv1.PluginStreamMessage) error {
	// Fail registration messages
	if register := msg.GetRegister(); register != nil {
		return fmt.Errorf("mock registration failure")
	}
	return nil
}

func (m *failingMockStream) Recv() (*pluginframeworkv1.PluginStreamMessage, error) {
	return nil, fmt.Errorf("mock stream closed")
}

func (m *failingMockStream) Context() context.Context {
	return m.ctx
}

// failingStreamCreator creates a stream that fails during registration
func failingStreamCreator(conn *grpc.ClientConn) (stream.StreamInterface, error) {
	return &failingMockStream{ctx: context.Background()}, nil
}

func TestNew_PluginStreamClientCreationFails(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	// Mock service descriptor and implementation
	mockServiceDesc := grpc.ServiceDesc{
		ServiceName: "test.TestService",
		Methods:     []grpc.MethodDesc{},
		Streams:     []grpc.StreamDesc{},
	}
	var mockImpl interface{} = &mockServiceImpl{}

	_, err := New(ctx, "test-plugin", "unix:///tmp/test.sock", "v1.0.0", mockServiceDesc, mockImpl, failingStreamCreator)

	if err == nil {
		t.Error("New() expected error when PluginStreamClient creation fails, got nil")
	}

	// Verify the error message contains the expected failure reason
	if !strings.Contains(err.Error(), "failed to create plugin stream client") {
		t.Errorf("New() error = %q, expected to contain 'failed to create plugin stream client'", err.Error())
	}
}
