package transport

import (
	"fmt"
	"time"
)

type Config struct {
	Protocol ProtocolType
	Target   string
	Timeout  time.Duration
	TLS      bool
}

func NewTransport(config Config) (Transport, error) {
	switch config.Protocol {
	case HTTP1:
		return NewHTTP1Transport(config.Target, config.Timeout), nil
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", config.Protocol)
	}
}
