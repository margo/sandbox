package types

// Final Margo Trust Bundle Map, conforming to SPIFFE Bundle Map
type MargoTrustBundleMap struct {
	TrustDomains MargoTrustBundle `json:"trust_domains"`
}

// map[trustDomain]trustBundle
type MargoTrustBundle map[string][]byte
