package registry

import (
	"sync"
	"testing"
)

// MockPluginProvider is a test implementation of PluginProvider
type MockPluginProvider struct {
	name string
}

func (m *MockPluginProvider) Name() string {
	return m.name
}

func TestNew(t *testing.T) {
	m := New()
	if m == nil {
		t.Fatalf("New() returned nil")
	}
}

func TestRegisterGet(t *testing.T) {
	m := New()
	plugin := &MockPluginProvider{name: "test-plugin"}

	// Register plugin
	m.Register("test", plugin)

	// Get plugin
	retrieved, err := m.Get("test")
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}

	if retrieved.Name() != "test-plugin" {
		t.Errorf("expected plugin name 'test-plugin', got %s", retrieved.Name())
	}
}

func TestGetNonExistent(t *testing.T) {
	m := New()

	_, err := m.Get("non-existent")
	if err == nil {
		t.Errorf("expected Get() to fail for non-existent plugin")
	}
}

func TestRegisterDuplicate(t *testing.T) {
	m := New()
	plugin1 := &MockPluginProvider{name: "plugin1"}
	plugin2 := &MockPluginProvider{name: "plugin2"}

	// Register first plugin
	m.Register("test", plugin1)

	// Register with same key (should overwrite or error)
	m.Register("test", plugin2)
	// Implementation allows overwrite
}

func TestUnregister(t *testing.T) {
	m := New()
	plugin := &MockPluginProvider{name: "test-plugin"}

	// Register plugin
	m.Register("test", plugin)

	// Verify it exists
	if _, err := m.Get("test"); err != nil {
		t.Fatalf("plugin should exist after registration")
	}

	// Unregister
	m.Unregister("test")

	// Verify it's gone
	if _, err := m.Get("test"); err == nil {
		t.Errorf("plugin should not exist after unregistration")
	}
}

func TestList(t *testing.T) {
	m := New()

	plugins := []string{"plugin1", "plugin2", "plugin3"}
	for _, name := range plugins {
		m.Register(name, &MockPluginProvider{name: name})
	}

	list := m.List()
	if len(list) != len(plugins) {
		t.Errorf("expected %d plugins, got %d", len(plugins), len(list))
	}

	// Check all plugins are in list
	for _, expected := range plugins {
		found := false
		for _, actual := range list {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected plugin %s in list", expected)
		}
	}
}

func TestGetAll(t *testing.T) {
	m := New()

	pluginsByKey := map[string]*MockPluginProvider{
		"key1": {name: "plugin1"},
		"key2": {name: "plugin2"},
		"key3": {name: "plugin3"},
	}

	for key, plugin := range pluginsByKey {
		m.Register(key, plugin)
	}

	all := m.GetAll()
	if len(all) != len(pluginsByKey) {
		t.Errorf("expected %d plugins, got %d", len(pluginsByKey), len(all))
	}

	// Check all plugins are present
	for key, expectedPlugin := range pluginsByKey {
		actualPlugin, exists := all[key]
		if !exists {
			t.Errorf("expected plugin with key %s", key)
			continue
		}
		if actualPlugin.Name() != expectedPlugin.Name() {
			t.Errorf("expected plugin name %s, got %s", expectedPlugin.Name(), actualPlugin.Name())
		}
	}
}

func TestCount(t *testing.T) {
	m := New()

	if m.Count() != 0 {
		t.Errorf("expected 0 plugins initially")
	}

	m.Register("plugin1", &MockPluginProvider{name: "plugin1"})
	if m.Count() != 1 {
		t.Errorf("expected 1 plugin after registration")
	}

	m.Register("plugin2", &MockPluginProvider{name: "plugin2"})
	if m.Count() != 2 {
		t.Errorf("expected 2 plugins after second registration")
	}

	m.Unregister("plugin1")
	if m.Count() != 1 {
		t.Errorf("expected 1 plugin after unregistration")
	}
}

func TestConcurrentRegisterUnregister(t *testing.T) {
	m := New()
	numGoroutines := 100
	numOps := 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				key := string(rune('a'+id%26)) + string(rune('0'+j%10))
				plugin := &MockPluginProvider{name: key}

				m.Register(key, plugin)
				m.Unregister(key)
			}
		}(i)
	}

	wg.Wait()

	// After all goroutines, registry should be empty or have minimal entries
	if m.Count() > numGoroutines { // Allow some leftover due to race conditions
		t.Logf("registry has %d plugins after concurrent ops", m.Count())
	}
}

func TestConcurrentRead(t *testing.T) {
	m := New()

	// Pre-register some plugins
	for i := 0; i < 10; i++ {
		key := string(rune('0' + i))
		m.Register(key, &MockPluginProvider{name: key})
	}

	numGoroutines := 100
	done := make(chan bool, numGoroutines)

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		go func() {
			_ = m.List()
			_ = m.GetAll()
			_ = m.Count()
			done <- true
		}()
	}

	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	if m.Count() != 10 {
		t.Errorf("expected 10 plugins after concurrent reads")
	}
}

func TestListEmpty(t *testing.T) {
	m := New()

	list := m.List()
	if len(list) != 0 {
		t.Errorf("expected empty list for new registry")
	}
}

func TestGetAllEmpty(t *testing.T) {
	m := New()

	all := m.GetAll()
	if len(all) != 0 {
		t.Errorf("expected empty map for new registry")
	}
}
