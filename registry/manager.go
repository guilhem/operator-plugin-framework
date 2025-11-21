package registry

import (
	"errors"
	"sync"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

// PluginProvider defines the interface for plugin implementations.
type PluginProvider interface {
	Name() string
}

// Manager manages plugin registration and retrieval.
type Manager struct {
	plugins map[string]PluginProvider
	mu      sync.RWMutex
}

// New creates a new plugin manager.
func New() *Manager {
	return &Manager{
		plugins: make(map[string]PluginProvider),
	}
}

// Register registers a plugin with the manager.
func (m *Manager) Register(name string, provider PluginProvider) {
	m.mu.Lock()
	defer m.mu.Unlock()

	log := log.Log
	log.Info("Registering plugin", "name", name)

	m.plugins[name] = provider
}

// Unregister removes a plugin from the manager.
func (m *Manager) Unregister(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	log := log.Log
	log.Info("Unregistering plugin", "name", name)

	delete(m.plugins, name)
}

// Get returns a plugin by name.
func (m *Manager) Get(name string) (PluginProvider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plugin, exists := m.plugins[name]
	if !exists {
		return nil, errors.New("plugin not found")
	}

	return plugin, nil
}

// GetAll returns a copy of all registered plugins.
func (m *Manager) GetAll() map[string]PluginProvider {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]PluginProvider)
	for name, plugin := range m.plugins {
		result[name] = plugin
	}
	return result
}

// List returns a list of all registered plugin names.
func (m *Manager) List() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.plugins))
	for name := range m.plugins {
		names = append(names, name)
	}
	return names
}

// Count returns the number of registered plugins.
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.plugins)
}
