package api

// PayloadSignature is the generated security value container expected by
// oas_security_gen.go. The current generated package is missing this type, so
// we provide the minimal shape used by the server and client security hooks.
type PayloadSignature struct {
	APIKey string
	Roles  []string
}
