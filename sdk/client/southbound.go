package client

import (
	"github.com/margo/dev-repo/sdk/auth"
	"github.com/margo/dev-repo/sdk/transport"
)

type SouthboundClient struct {
	auth      auth.Authenticator
	transport transport.Transport
}

func NewSouthboundClient(auth auth.Authenticator, transport transport.Transport) *SouthboundClient {
	return &SouthboundClient{
		auth:      auth,
		transport: transport,
	}
}

func (south *SouthboundClient) Poke() error {
	return nil
}

func (south *SouthboundClient) Poll() error {
	return nil
}

func (south *SouthboundClient) pollNewApps() error {
	return nil
}

func (south *SouthboundClient) pollDeletedApps() error {
	return nil
}

func (south *SouthboundClient) pollAppChanges() error {
	return nil
}
