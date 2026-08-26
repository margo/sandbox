package types

import "github.com/margo/sandbox/mis/pkg/standard/generatedCode"

type MISIface interface {
	GetDiscoveryDocument() *generatedCode.DiscoveryDocument
	GetTrustBundle()
}
