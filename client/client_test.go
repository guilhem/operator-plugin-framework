package client

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		opts    []ClientOption
		wantErr bool
	}{
		{
			name:    "unix socket - no connection",
			addr:    "unix:///tmp/test.sock",
			wantErr: false, // Connection will fail at dial time, but New succeeds
		},
		{
			name:    "tcp address - no connection",
			addr:    "tcp://localhost:50051",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			conn, err := New(ctx, "test-plugin", tt.addr, tt.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if conn != nil && !tt.wantErr {
				if err := conn.Close(); err != nil {
					t.Logf("Close() error: %v", err)
				}
			}
		})
	}
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
	provider := NewServiceAccountTokenProviderWithPath(tokenPath)
	token, err := provider.GetToken()
	if err != nil {
		t.Fatalf("GetToken() error = %v", err)
	}
	if token != expectedToken {
		t.Errorf("GetToken() = %q, want %q", token, expectedToken)
	}
}

func TestServiceAccountTokenProvider_FileNotFound(t *testing.T) {
	provider := NewServiceAccountTokenProviderWithPath("/nonexistent/token")
	_, err := provider.GetToken()
	if err == nil {
		t.Error("GetToken() expected error for nonexistent file, got nil")
	}
}

func TestStaticTokenProvider(t *testing.T) {
	expectedToken := "static-token-xyz"
	provider := NewStaticTokenProvider(expectedToken)

	token, err := provider.GetToken()
	if err != nil {
		t.Fatalf("GetToken() error = %v", err)
	}
	if token != expectedToken {
		t.Errorf("GetToken() = %q, want %q", token, expectedToken)
	}
}

func TestWithTokenProvider(t *testing.T) {
	provider := NewStaticTokenProvider("test-token")

	conn := &Connection{}
	opt := WithTokenProvider(provider)
	opt(conn)

	if conn.tokenProvider != provider {
		t.Error("WithTokenProvider() did not set token provider")
	}
}

func TestWithServiceAccountToken(t *testing.T) {
	conn := &Connection{}
	opt := WithServiceAccountToken()
	opt(conn)

	if conn.tokenProvider == nil {
		t.Error("WithServiceAccountToken() did not set token provider")
	}

	// Verify it's a ServiceAccountTokenProvider with default path
	saProvider, ok := conn.tokenProvider.(*ServiceAccountTokenProvider)
	if !ok {
		t.Errorf("WithServiceAccountToken() set wrong provider type: %T", conn.tokenProvider)
	}
	if saProvider.tokenPath != "/var/run/secrets/kubernetes.io/serviceaccount/token" {
		t.Errorf("WithServiceAccountToken() wrong path = %q", saProvider.tokenPath)
	}
}

func TestWithServiceAccountTokenPath(t *testing.T) {
	customPath := "/custom/path/token"
	conn := &Connection{}
	opt := WithServiceAccountTokenPath(customPath)
	opt(conn)

	if conn.tokenProvider == nil {
		t.Error("WithServiceAccountTokenPath() did not set token provider")
	}

	saProvider, ok := conn.tokenProvider.(*ServiceAccountTokenProvider)
	if !ok {
		t.Errorf("WithServiceAccountTokenPath() set wrong provider type: %T", conn.tokenProvider)
	}
	if saProvider.tokenPath != customPath {
		t.Errorf("WithServiceAccountTokenPath() path = %q, want %q", saProvider.tokenPath, customPath)
	}
}

func TestWithStaticToken(t *testing.T) {
	expectedToken := "my-static-token"
	conn := &Connection{}
	opt := WithStaticToken(expectedToken)
	opt(conn)

	if conn.tokenProvider == nil {
		t.Error("WithStaticToken() did not set token provider")
	}

	token, err := conn.tokenProvider.GetToken()
	if err != nil {
		t.Fatalf("GetToken() error = %v", err)
	}
	if token != expectedToken {
		t.Errorf("GetToken() = %q, want %q", token, expectedToken)
	}
}

func TestTokenAuth_GetRequestMetadata(t *testing.T) {
	auth := &tokenAuth{token: "test-token-123"}

	metadata, err := auth.GetRequestMetadata(context.Background())
	if err != nil {
		t.Fatalf("GetRequestMetadata() error = %v", err)
	}

	expectedAuth := "Bearer test-token-123"
	if metadata["authorization"] != expectedAuth {
		t.Errorf("authorization header = %q, want %q", metadata["authorization"], expectedAuth)
	}
}

func TestTokenAuth_RequireTransportSecurity(t *testing.T) {
	auth := &tokenAuth{token: "test"}

	// Should return false since kube-rbac-proxy handles TLS
	if auth.RequireTransportSecurity() {
		t.Error("RequireTransportSecurity() = true, want false")
	}
}

func TestConnectionName(t *testing.T) {
	name := "test-plugin"
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	conn, err := New(ctx, name, "unix:///tmp/test.sock")
	if err == nil {
		defer conn.Close()
		if conn.Name() != name {
			t.Errorf("expected Name()=%s, got %s", name, conn.Name())
		}
	}
}

func TestConnectionWithToken(t *testing.T) {
	// Create temp token file
	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, "token")
	expectedToken := "test-token-xyz"

	if err := os.WriteFile(tokenPath, []byte(expectedToken), 0600); err != nil {
		t.Fatalf("failed to create test token: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	conn, err := New(ctx, "test-plugin", "unix:///tmp/test.sock",
		WithServiceAccountTokenPath(tokenPath),
	)
	if err == nil {
		defer conn.Close()

		// Verify token provider is set
		if conn.tokenProvider == nil {
			t.Error("expected tokenProvider to be set")
		}

		// Verify token can be retrieved
		token, err := conn.tokenProvider.GetToken()
		if err != nil {
			t.Errorf("GetToken() error = %v", err)
		}
		if token != expectedToken {
			t.Errorf("GetToken() = %q, want %q", token, expectedToken)
		}
	}
}

func TestNew_WithInvalidToken(t *testing.T) {
	// Provider that returns an error
	provider := NewServiceAccountTokenProviderWithPath("/nonexistent/token")

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := New(ctx, "test-plugin", "unix:///tmp/test.sock",
		WithTokenProvider(provider),
	)

	if err == nil {
		t.Error("New() expected error when token provider fails, got nil")
	}
}
