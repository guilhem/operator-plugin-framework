# Operator Plugin Framework

A reusable gRPC-based plugin framework for Kubernetes operators with secure plugin communication via kube-rbac-proxy.

## Features

- **Bidirectional gRPC Streaming**: Full-duplex communication between operators and plugins
- **Automatic Plugin Registration**: Plugins register on connection via streaming RPC
- **Kubernetes-Native Security**: ServiceAccount token authentication via kube-rbac-proxy
- **Controller-Runtime Integration**: Server implements `Runnable` interface for seamless lifecycle management

## Architecture

This framework provides **two separate services**:

1. **PluginFrameworkService** (Framework): Handles plugin communication and registration
2. **Your Application Service** (Your Code): Defines your business logic RPC methods

## Quick Start

### For Operators

```go
import "github.com/guilhem/operator-plugin-framework/server"

// Server implements controller-runtime Runnable interface
func setupPluginServer(mgr ctrl.Manager) error {
    s := server.New("unix:///tmp/plugins.sock")
    
    // Add to manager - server lifecycle managed automatically
    return mgr.Add(s)  // Server.Start() called when manager starts
}
```

### For Plugins

```go
import "github.com/guilhem/operator-plugin-framework/client"

// Framework handles PluginFrameworkService connection automatically
conn, err := client.New(
    ctx,
    "my-plugin",
    "https://operator-kube-rbac-proxy:8443",
    "v1.0.0",
    &pb.MyService_ServiceDesc,
    &myServiceImpl{},
    client.WithServiceAccountToken(),
)
```

## API Reference

### Server (Implements Runnable)

```go
// New creates a plugin server
func New(addr string, opts ...ServerOption) *Server

// Start implements controller-runtime Runnable interface
// Called automatically when added to manager with mgr.Add(server)
func (s *Server) Start(ctx context.Context) error

// GetRegistry returns the plugin registry
func (s *Server) GetRegistry() *registry.Manager
```

### Client

```go
// New creates authenticated gRPC connection to operator
// Automatically handles PluginFrameworkService for stream communication
func New(ctx context.Context, name string, target string, pluginVersion string, serviceDesc grpc.ServiceDesc, impl any, opts ...ClientOption) (*Client, error)

// Authentication options:
// WithServiceAccountToken() - Default Kubernetes ServiceAccount tokens
// WithStaticToken(token) - For testing
```

### Registry

```go
// Get retrieves a plugin by name
func (m *Manager) Get(name string) (PluginProvider, error)

// List returns all registered plugins
func (m *Manager) List() map[string]PluginProvider
```

## Usage in Controller

```go
func (r *MyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // Get plugin from registry
    plugin, err := r.PluginRegistry.Get(resource.Spec.ProviderName)
    if err != nil {
        return ctrl.Result{}, fmt.Errorf("plugin not found: %w", err)
    }
    
    // Call plugin RPC method
    result, err := plugin.DoSomething(ctx, request)
    // ... handle result
}
```

## Security

- **kube-rbac-proxy**: Validates ServiceAccount tokens and enforces RBAC
- **TLS**: All communication encrypted via HTTPS
- **RBAC**: Separate permissions for operator and plugin access

## Dependencies

- `google.golang.org/grpc`: gRPC framework
- `sigs.k8s.io/controller-runtime`: Kubernetes operator framework
- `google.golang.org/protobuf`: Protocol Buffers

## License

Apache License 2.0
