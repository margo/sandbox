package crypto
import (
    "crypto/tls"
    "crypto/x509"
    "fmt"
    "os"
)
 
// LoadCustomCA loads a custom CA certificate and returns a TLS config
func LoadCustomCA(caPath string) (*tls.Config, error) {
    // Read the CA certificate file
    caCert, err := os.ReadFile(caPath)
    if err != nil {
        return nil, fmt.Errorf("failed to read CA certificate from %s: %w", caPath, err)
    }
 
    // Create a certificate pool and add the CA
    caCertPool := x509.NewCertPool()
    if !caCertPool.AppendCertsFromPEM(caCert) {
        return nil, fmt.Errorf("failed to parse CA certificate from %s", caPath)
    }
 
    // Create TLS config with the custom CA
    tlsConfig := &tls.Config{
        RootCAs: caCertPool,
    }
 
    return tlsConfig, nil
}