package transport

import (
	"context"
	"io"
)

// Transport defines the common interface for all transport protocols
type Transport interface {
	// Send sends a request and returns a response
	Send(ctx context.Context, req *Request) (*Response, error)

	// Stream creates a bidirectional stream (for streaming protocols)
	Stream(ctx context.Context) (Stream, error)

	// Close closes the transport connection
	Close() error

	// Protocol returns the transport protocol type
	Protocol() ProtocolType
}

// Request represents a generic transport request
type Request struct {
	Method   string
	Path     string
	Headers  map[string]string
	Body     io.Reader
	Metadata map[string]interface{}
}

// Response represents a generic transport response
type Response struct {
	StatusCode int
	Headers    map[string]string
	Body       io.Reader
	Metadata   map[string]interface{}
}

// Stream interface for bidirectional communication
type Stream interface {
	Send(*Request) error
	Recv() (*Response, error)
	Close() error
}

type ProtocolType string

const (
	HTTP1 ProtocolType = "http1.1"
	HTTP2 ProtocolType = "http2.0"
	HTTP3 ProtocolType = "http3.0"
)
