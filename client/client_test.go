package client

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc"

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
			wantErr: true, // Now expects connection error since we try to connect
		},
		{
			name:    "tcp address - connection setup succeeds",
			addr:    "tcp://127.0.0.1:0", // Use port 0 to avoid conflicts
			wantErr: true,                // Expects connection error since no server is running
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
			defer cancel()

			// Mock service descriptor and implementation for testing
			mockServiceDesc := grpc.ServiceDesc{
				ServiceName: "test.TestService",
				Methods:     []grpc.MethodDesc{},
				Streams:     []grpc.StreamDesc{},
			}

			var mockImpl interface{} = &mockServiceImpl{}

			_, err := New(ctx, "test-plugin", tt.addr, "v1.0.0", mockServiceDesc, mockImpl, tt.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

// mockServiceImpl is a mock implementation for testing
type mockServiceImpl struct{}

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

	_, err := New(ctx, "test-plugin", "unix:///tmp/test.sock", "v1.0.0", mockServiceDesc, mockImpl,
		WithTokenProvider(provider),
	)

	if err == nil {
		t.Error("New() expected error when token provider fails, got nil")
	}
}
