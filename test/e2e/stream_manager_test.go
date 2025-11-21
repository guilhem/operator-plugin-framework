package e2e

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/guilhem/operator-plugin-framework/server"
)

// TestStreamManagerBasicRegistration tests basic plugin registration and unregistration
func TestStreamManagerBasicRegistration(t *testing.T) {
	tmpDir := t.TempDir()
	sockPath := filepath.Join(tmpDir, "stream-manager.sock")
	addr := fmt.Sprintf("unix:///%s", sockPath)

	s := server.New(addr)
	sm := s.GetStreamManager()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start server
	errs := make(chan error, 1)
	go func() {
		errs <- s.Start(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Test 1: Register a plugin via HandlePluginStream
	pluginCtx, pluginCancel := context.WithCancel(context.Background())
	pluginName := "test-plugin"

	go func() {
		_ = sm.HandlePluginStream(pluginCtx, pluginName)
	}()

	time.Sleep(50 * time.Millisecond)

	// Verify plugin is registered
	if !sm.IsPluginConnected(pluginName) {
		t.Fatal("plugin should be registered")
	}

	if sm.ConnectionCount() != 1 {
		t.Errorf("expected 1 connection, got %d", sm.ConnectionCount())
	}

	// Test 2: Disconnect plugin
	pluginCancel()
	time.Sleep(50 * time.Millisecond)

	// Verify plugin is unregistered
	if sm.IsPluginConnected(pluginName) {
		t.Fatal("plugin should be unregistered")
	}

	if sm.ConnectionCount() != 0 {
		t.Errorf("expected 0 connections, got %d", sm.ConnectionCount())
	}

	cancel()
	<-errs
}

// TestStreamManagerConcurrentRegistration tests multiple plugins registering simultaneously
func TestStreamManagerConcurrentRegistration(t *testing.T) {
	tmpDir := t.TempDir()
	sockPath := filepath.Join(tmpDir, "stream-manager-concurrent.sock")
	addr := fmt.Sprintf("unix:///%s", sockPath)

	s := server.New(addr, server.WithMaxConnections(50))
	sm := s.GetStreamManager()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start server
	errs := make(chan error, 1)
	go func() {
		errs <- s.Start(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Simulate concurrent plugin registrations
	numPlugins := 20
	var wg sync.WaitGroup
	wg.Add(numPlugins)

	contexts := make([]context.CancelFunc, numPlugins)

	for i := 0; i < numPlugins; i++ {
		go func(id int) {
			defer wg.Done()
			pluginName := fmt.Sprintf("plugin-%d", id)

			pluginCtx, cancel := context.WithCancel(context.Background())
			contexts[id] = cancel

			// Register plugin via HandlePluginStream
			_ = sm.HandlePluginStream(pluginCtx, pluginName)
		}(i)
	}

	// Give plugins time to register
	time.Sleep(200 * time.Millisecond)

	// Verify all plugins are registered
	if sm.ConnectionCount() != numPlugins {
		t.Errorf("expected %d connections, got %d", numPlugins, sm.ConnectionCount())
	}

	plugins := sm.ListConnectedPlugins()
	if len(plugins) != numPlugins {
		t.Errorf("expected %d plugins in list, got %d", numPlugins, len(plugins))
	}

	// Disconnect all plugins
	for _, cancel := range contexts {
		if cancel != nil {
			cancel()
		}
	}

	wg.Wait()
	time.Sleep(100 * time.Millisecond)

	// Verify all plugins unregistered
	if sm.ConnectionCount() != 0 {
		t.Errorf("expected 0 connections after disconnection, got %d", sm.ConnectionCount())
	}

	cancel()
	<-errs
}

// TestStreamManagerMaxConnections tests max connections limit enforcement
func TestStreamManagerMaxConnections(t *testing.T) {
	tmpDir := t.TempDir()
	sockPath := filepath.Join(tmpDir, "stream-manager-maxconn.sock")
	addr := fmt.Sprintf("unix:///%s", sockPath)

	maxConnections := 5
	s := server.New(addr, server.WithMaxConnections(maxConnections))
	sm := s.GetStreamManager()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start server
	errs := make(chan error, 1)
	go func() {
		errs <- s.Start(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Register up to max connections
	contexts := make([]context.CancelFunc, maxConnections)
	for i := 0; i < maxConnections; i++ {
		pluginName := fmt.Sprintf("plugin-%d", i)
		pluginCtx, pluginCancel := context.WithCancel(t.Context())
		contexts[i] = pluginCancel

		go func() {
			_ = sm.HandlePluginStream(pluginCtx, pluginName)
		}()
	}

	time.Sleep(100 * time.Millisecond)

	if sm.ConnectionCount() != maxConnections {
		t.Errorf("expected %d connections, got %d", maxConnections, sm.ConnectionCount())
	}

	// Try to register one more (should fail)
	pluginCtx, _ := context.WithCancel(t.Context())
	err := sm.HandlePluginStream(pluginCtx, "extra-plugin")
	if err != server.ErrMaxConnectionsReached {
		t.Errorf("expected ErrMaxConnectionsReached, got %v", err)
	}

	// Cleanup
	for _, cancel := range contexts {
		if cancel != nil {
			cancel()
		}
	}

	cancel()
	<-errs
}

// TestStreamManagerConnectionInfo tests GetPluginInfo for connection metadata
func TestStreamManagerConnectionInfo(t *testing.T) {
	tmpDir := t.TempDir()
	sockPath := filepath.Join(tmpDir, "stream-manager-info.sock")
	addr := fmt.Sprintf("unix:///%s", sockPath)

	s := server.New(addr)
	sm := s.GetStreamManager()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start server
	errs := make(chan error, 1)
	go func() {
		errs <- s.Start(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	pluginName := "info-test-plugin"
	pluginCtx, pluginCancel := context.WithCancel(context.Background())

	go func() {
		_ = sm.HandlePluginStream(pluginCtx, pluginName)
	}()

	time.Sleep(50 * time.Millisecond)

	// Get plugin info
	info := sm.GetPluginInfo(pluginName)
	if info == nil {
		t.Fatal("GetPluginInfo should return info for connected plugin")
	}

	if info.Name != pluginName {
		t.Errorf("expected plugin name %s, got %s", pluginName, info.Name)
	}

	if info.ConnectedAt.IsZero() {
		t.Error("ConnectedAt should not be zero")
	}

	if info.Uptime <= 0 {
		t.Error("Uptime should be positive")
	}

	pluginCancel()
	cancel()
	<-errs
}

// TestStreamManagerConnectionHandler tests the connection lifecycle handler
func TestStreamManagerConnectionHandler(t *testing.T) {
	tmpDir := t.TempDir()
	sockPath := filepath.Join(tmpDir, "stream-manager-handler.sock")
	addr := fmt.Sprintf("unix:///%s", sockPath)

	// Create a custom connection handler to track events
	connectEvents := []string{}
	disconnectEvents := []string{}
	mu := sync.Mutex{}

	customHandler := &testConnectionHandler{
		onConnect: func(name string) error {
			mu.Lock()
			connectEvents = append(connectEvents, name)
			mu.Unlock()
			return nil
		},
		onDisconnect: func(name string) error {
			mu.Lock()
			disconnectEvents = append(disconnectEvents, name)
			mu.Unlock()
			return nil
		},
	}

	s := server.New(addr)
	sm := server.NewStreamManager(s, server.WithConnectionHandler(customHandler))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start server
	errs := make(chan error, 1)
	go func() {
		errs <- s.Start(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Connect a plugin
	pluginName := "handler-test-plugin"
	pluginCtx, pluginCancel := context.WithCancel(context.Background())

	go func() {
		_ = sm.HandlePluginStream(pluginCtx, pluginName)
	}()

	time.Sleep(100 * time.Millisecond)

	// Verify connect handler was called
	mu.Lock()
	if len(connectEvents) != 1 || connectEvents[0] != pluginName {
		t.Errorf("expected connect event for %s, got %v", pluginName, connectEvents)
	}
	mu.Unlock()

	// Disconnect plugin
	pluginCancel()
	time.Sleep(100 * time.Millisecond)

	// Verify disconnect handler was called
	mu.Lock()
	if len(disconnectEvents) != 1 || disconnectEvents[0] != pluginName {
		t.Errorf("expected disconnect event for %s, got %v", pluginName, disconnectEvents)
	}
	mu.Unlock()

	cancel()
	<-errs
}

// testConnectionHandler is a test implementation of PluginConnectionHandler
type testConnectionHandler struct {
	onConnect    func(string) error
	onDisconnect func(string) error
}

func (h *testConnectionHandler) OnPluginConnect(name string) error {
	if h.onConnect != nil {
		return h.onConnect(name)
	}
	return nil
}

func (h *testConnectionHandler) OnPluginDisconnect(name string) error {
	if h.onDisconnect != nil {
		return h.onDisconnect(name)
	}
	return nil
}
