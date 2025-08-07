package git

// Auth holds authentication credentials for Git repository access.
//
// Note: For GitHub and similar services, use personal access tokens instead of passwords
// for enhanced security and to comply with authentication best practices.
type Auth struct {
	Username   string // Username for Git authentication
	Token      string // Personal access token or password for authentication
	CABundle   []byte // CA bundle (PEM encoded) for self-signed certificates
	ClientCert []byte // Client certificate (PEM encoded)
	ClientKey  []byte // Private key (PEM encoded) for client certificate
}
