// certs/pki/example.go
package pki

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

type AuthService struct {
	authenticator  *PKIAuthenticator
	challengeStore map[string]*Challenge // Use Redis in production
}

func NewAuthService(caPEMs [][]byte) (*AuthService, error) {
	certManager, err := NewCertificateManager(caPEMs)
	if err != nil {
		return nil, err
	}

	challengeGen := NewChallengeGenerator(32, 5*time.Minute)
	signatureVerifier := NewSignatureVerifier()

	authenticator := NewPKIAuthenticator(certManager, challengeGen, signatureVerifier)

	return &AuthService{
		authenticator:  authenticator,
		challengeStore: make(map[string]*Challenge),
	}, nil
}

// Step 1: Generate challenge for device
func (s *AuthService) CreateChallenge(deviceID string) (string, error) {
	challenge, err := s.authenticator.GenerateAuthenticationChallenge(deviceID)
	if err != nil {
		return "", fmt.Errorf("failed to generate challenge: %w", err)
	}

	// Store challenge (use Redis with TTL in production)
	s.challengeStore[deviceID] = challenge

	// Return base64 encoded challenge
	return base64.StdEncoding.EncodeToString(challenge.Value), nil
}

// Step 2: Verify device authentication
func (s *AuthService) VerifyAuthentication(deviceID string, certPEM []byte, signatureB64 string) (*AuthenticationResult, error) {
	// Get stored challenge
	challenge, exists := s.challengeStore[deviceID]
	if !exists {
		return &AuthenticationResult{
			DeviceID:     deviceID,
			ErrorMessage: "no challenge found for device",
		}, nil
	}

	// Decode signature
	signature, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		return &AuthenticationResult{
			DeviceID:     deviceID,
			ErrorMessage: "invalid signature encoding",
		}, nil
	}

	// Perform authentication
	result := s.authenticator.PerformFullAuthentication(certPEM, challenge, signature)

	// Clean up challenge on success or failure
	delete(s.challengeStore, deviceID)

	return result, nil
}

// Example server main function
func serverMain() {
	// Load CA certificates
	caPEM := []byte(`-----BEGIN CERTIFICATE-----
MIICxjCCAa4CAQAwDQYJKoZIhvcNAQELBQAwEzERMA8GA1UEAwwIVGVzdCBDQSAwHhcN...
-----END CERTIFICATE-----`)

	authService, err := NewAuthService([][]byte{caPEM})
	if err != nil {
		log.Fatal("Failed to create auth service:", err)
	}

	// Simulate authentication flow
	deviceID := "device-12345"

	// Step 1: Generate challenge
	challengeB64, err := authService.CreateChallenge(deviceID)
	if err != nil {
		log.Fatal("Failed to create challenge:", err)
	}

	fmt.Printf("Generated challenge for device %s: %s\n", deviceID, challengeB64)

	// Step 2: Verify authentication (would come from client)
	// This would typically be called from your HTTP handler
	deviceCertPEM := []byte(`-----BEGIN CERTIFICATE-----...-----END CERTIFICATE-----`)
	signatureFromClient := "base64-encoded-signature-from-client"

	result, err := authService.VerifyAuthentication(deviceID, deviceCertPEM, signatureFromClient)
	if err != nil {
		log.Fatal("Authentication error:", err)
	}

	if result.Success {
		fmt.Printf("Device %s authenticated successfully!\n", result.DeviceID)
		// Generate JWT token, create session, etc.
	} else {
		fmt.Printf("Authentication failed: %s\n", result.ErrorMessage)
	}
}

// ----------------------------------------------------------------------------
// ----------------------------------------------------------------------------
// ----------------------------------------------------------------------------
// certs/pki/example.go

type DeviceClient struct {
	pkiClient  *PKIClient
	serverURL  string
	httpClient *http.Client
}

func NewDeviceClient(deviceID string, privateKeyPEM, certPEM []byte, serverURL string) (*DeviceClient, error) {
	pkiClient, err := NewPKIClient(deviceID, privateKeyPEM, certPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to create PKI client: %w", err)
	}

	return &DeviceClient{
		pkiClient:  pkiClient,
		serverURL:  serverURL,
		httpClient: &http.Client{},
	}, nil
}

// Step 1: Request challenge from server
func (c *DeviceClient) RequestChallenge() (string, error) {
	url := fmt.Sprintf("%s/auth/challenge?device_id=%s", c.serverURL, c.pkiClient.GetDeviceID())

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to request challenge: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("server returned status: %d", resp.StatusCode)
	}

	var challengeResp struct {
		Challenge string `json:"challenge"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&challengeResp); err != nil {
		return "", fmt.Errorf("failed to decode challenge response: %w", err)
	}

	return challengeResp.Challenge, nil
}

// Step 2: Authenticate with server
func (c *DeviceClient) Authenticate() (*AuthResult, error) {
	// Get challenge from server
	challengeB64, err := c.RequestChallenge()
	if err != nil {
		return nil, fmt.Errorf("failed to get challenge: %w", err)
	}

	// Decode challenge
	challengeData, err := base64.StdEncoding.DecodeString(challengeB64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode challenge: %w", err)
	}

	// Sign challenge
	signature, err := c.pkiClient.SignChallenge(challengeData)
	if err != nil {
		return nil, fmt.Errorf("failed to sign challenge: %w", err)
	}

	// Prepare authentication request
	authReq := AuthRequest{
		DeviceID:    c.pkiClient.GetDeviceID(),
		Challenge:   challengeB64,
		Signature:   base64.StdEncoding.EncodeToString(signature),
		Certificate: base64.StdEncoding.EncodeToString(c.pkiClient.GetCertificatePEM()),
	}

	// Send authentication request
	return c.sendAuthRequest(authReq)
}

func (c *DeviceClient) sendAuthRequest(authReq AuthRequest) (*AuthResult, error) {
	jsonData, err := json.Marshal(authReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal auth request: %w", err)
	}

	url := fmt.Sprintf("%s/auth/verify", c.serverURL)
	resp, err := c.httpClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to send auth request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var authResult AuthResult
	if err := json.Unmarshal(body, &authResult); err != nil {
		return nil, fmt.Errorf("failed to decode auth response: %w", err)
	}

	return &authResult, nil
}

// Data structures for API communication
type AuthRequest struct {
	DeviceID    string `json:"device_id"`
	Challenge   string `json:"challenge"`
	Signature   string `json:"signature"`
	Certificate string `json:"certificate"`
}

type AuthResult struct {
	Success bool   `json:"success"`
	Token   string `json:"token,omitempty"`
	Error   string `json:"error,omitempty"`
}

// Example client main function
func clientMain() {
	// Load device credentials
	deviceID := "device-12345"
	privateKeyPEM := []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA...
-----END RSA PRIVATE KEY-----`)

	certPEM := []byte(`-----BEGIN CERTIFICATE-----
MIICxjCCAa4CAQAwDQYJKoZIhvcNAQELBQAwEzERMA8GA1UEAwwIVGVzdCBDQSAwHhcN...
-----END CERTIFICATE-----`)

	// Create device client
	client, err := NewDeviceClient(deviceID, privateKeyPEM, certPEM, "https://api.example.com")
	if err != nil {
		log.Fatal("Failed to create device client:", err)
	}

	// Authenticate with server
	result, err := client.Authenticate()
	if err != nil {
		log.Fatal("Authentication failed:", err)
	}

	if result.Success {
		fmt.Printf("Authentication successful! Token: %s\n", result.Token)
		// Use token for subsequent API calls
	} else {
		fmt.Printf("Authentication failed: %s\n", result.Error)
	}
}
