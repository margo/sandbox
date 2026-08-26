package types

import (
	"github.com/margo/sandbox/mis/pkg/standard/generatedCode"
	"github.com/spiffe/go-spiffe/v2/bundle/spiffebundle"
)

type MISIface interface {
	GetDiscoveryDocument() *generatedCode.DiscoveryDocument
	GetTrustBundle() (*spiffebundle.Bundle, error)
}
