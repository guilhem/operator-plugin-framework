package token

import (
	"context"
	"fmt"
	"os"
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

// TokenCredential implements credentials.PerRPCCredentials for bearer token authentication
type TokenCredential struct {
	Provider TokenProvider
}

func (t *TokenCredential) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	token, err := t.Provider.GetToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	return map[string]string{
		"authorization": "Bearer " + token,
	}, nil
}

func (t *TokenCredential) RequireTransportSecurity() bool {
	return false // kube-rbac-proxy handles TLS
}
