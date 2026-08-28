package operations

import (
	"fmt"
	"strings"

	"github.com/margo/sandbox/mis/pkg/standard/generatedCode"
)

func (o *Operation) GetDiscoveryDocument() *generatedCode.DiscoveryDocument {
	// addr is of format :8443 -> [0]->"" , [1]->8443
	port := strings.Split(o.addr, ":")[1]
	tbu := fmt.Sprintf(
		"https://mis.%s:%s/%s",
		o.trustDomain,
		port,
		strings.Trim(o.trustBundleURI, "/"),
	)
	// If port is general usage port, no need to include it
	if port == "443" {
		tbu = fmt.Sprintf(
			"https://mis.%s/%s",
			o.trustDomain,
			strings.Trim(o.trustBundleURI, "/"),
		)
	}

	return &generatedCode.DiscoveryDocument{
		TrustBundleUri: tbu,
		TrustDomain:    o.trustDomain,
	}
}
