package server

import "errors"

var (
	ErrNotImplemented        = errors.New("not implemented: see ../../../internal/pluginserver for full implementation")
	ErrMaxConnectionsReached = errors.New("max plugin connections reached")
	ErrAuthenticationFailed  = errors.New("authentication failed")
	ErrPluginNotFound        = errors.New("plugin not found")
	ErrInvalidAddress        = errors.New("invalid server address")
	ErrServerNotRunning      = errors.New("server not running")
)
