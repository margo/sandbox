package operations

import (
	"fmt"
	"strings"

	"github.com/margo/sandbox/mis/pkg/standard/generatedCode"
)

func (o *Operation) GetDiscoveryDocument() *generatedCode.DiscoveryDocument {
	return &generatedCode.DiscoveryDocument{
		TrustBundleUri: fmt.Sprintf(
			"https://mis.%s/%s",
			o.trustDomain,
			strings.Trim(o.trustBundleURI, "/"),
		),
		TrustDomain: o.trustDomain,
	}
}
