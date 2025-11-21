package server

// ServerOption is a functional option for Server configuration
type ServerOption func(*Server)

// WithMaxConnections sets the maximum number of concurrent plugin connections
func WithMaxConnections(max int) ServerOption {
	return func(s *Server) {
		s.maxConnections = max
	}
}
