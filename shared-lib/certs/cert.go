package certs

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"time"
)

// --- Types matching OpenAPI spec ---
type PKIParameters struct {
	CertificateAuthority string `json:"certificateAuthority"`
	KeySize              int    `json:"keySize"`
	Algorithm            string `json:"algorithm"`
	CSR                  string `json:"csr"`
}

type OnboardingRequest struct {
	DeviceInfo struct {
		DeviceId string                 `json:"deviceId"`
		Metadata map[string]interface{} `json:"metadata"`
	} `json:"deviceInfo"`
	Protocol struct {
		Type       string        `json:"type"`
		Version    string        `json:"version"`
		Parameters PKIParameters `json:"parameters"`
	} `json:"protocol"`
	Metadata map[string]interface{} `json:"metadata"`
}

type OnboardingResponse struct {
	SessionId string `json:"sessionId"`
	DeviceId  string `json:"deviceId"`
	Status    string `json:"status"`
	NextStep  struct {
		Action   string `json:"action"`
		Endpoint string `json:"endpoint"`
	} `json:"nextStep"`
	ExpiresAt string `json:"expiresAt"`
}

type ChallengeRequest struct {
	SessionId string      `json:"sessionId"`
	Response  PKIResponse `json:"response"`
}

type PKIResponse struct {
	Certificate     string `json:"certificate"`
	PrivateKeyProof string `json:"privateKeyProof"`
}

type ChallengeResponse struct {
	SessionId string `json:"sessionId"`
	Status    string `json:"status"`
}

// --- Client-side PKI onboarding ---
func generateKeyAndCSR(deviceId string) (*rsa.PrivateKey, string, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, "", err
	}
	template := x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName: deviceId,
		},
	}
	csrBytes, err := x509.CreateCertificateRequest(rand.Reader, &template, priv)
	if err != nil {
		return nil, "", err
	}
	csrPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrBytes})
	return priv, base64.StdEncoding.EncodeToString(csrPEM), nil
}

func clientOnboard(serverURL, deviceId string) (string, *rsa.PrivateKey, error) {
	priv, csr, err := generateKeyAndCSR(deviceId)
	if err != nil {
		return "", nil, err
	}
	req := OnboardingRequest{
		DeviceInfo: struct {
			DeviceId string                 `json:"deviceId"`
			Metadata map[string]interface{} `json:"metadata"`
		}{
			DeviceId: deviceId,
			Metadata: map[string]interface{}{"annotations": map[string]string{"model": "x1"}},
		},
		Protocol: struct {
			Type       string        `json:"type"`
			Version    string        `json:"version"`
			Parameters PKIParameters `json:"parameters"`
		}{
			Type:    "PKI",
			Version: "1.0",
			Parameters: PKIParameters{
				CertificateAuthority: "TestCA",
				KeySize:              2048,
				Algorithm:            "RSA",
				CSR:                  csr,
			},
		},
	}
	body, _ := json.Marshal(req)
	resp, err := http.Post(serverURL+"/devices/onboard", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	var onboardResp OnboardingResponse
	json.NewDecoder(resp.Body).Decode(&onboardResp)
	return onboardResp.SessionId, priv, nil
}

func clientChallenge(serverURL, sessionId string, priv *rsa.PrivateKey, challenge []byte, certPEM string) error {
	// Sign challenge
	sig, err := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, challenge)
	if err != nil {
		return err
	}
	req := ChallengeRequest{
		SessionId: sessionId,
		Response: PKIResponse{
			Certificate:     certPEM,
			PrivateKeyProof: base64.StdEncoding.EncodeToString(sig),
		},
	}
	body, _ := json.Marshal(req)
	resp, err := http.Post(serverURL+"/devices/"+sessionId+"/onboard/challenge", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var challengeResp ChallengeResponse
	json.NewDecoder(resp.Body).Decode(&challengeResp)
	if challengeResp.Status != "passed" {
		return fmt.Errorf("challenge failed")
	}
	return nil
}

// --- Server-side handlers ---
func handleOnboard(w http.ResponseWriter, r *http.Request) {
	var req OnboardingRequest
	json.NewDecoder(r.Body).Decode(&req)
	csrBytes, _ := base64.StdEncoding.DecodeString(req.Protocol.Parameters.CSR)
	block, _ := pem.Decode(csrBytes)
	csr, _ := x509.ParseCertificateRequest(block.Bytes)
	// Issue certificate (self-signed for demo)
	template := x509.Certificate{
		SerialNumber:          bigInt(time.Now().UnixNano()),
		Subject:               csr.Subject,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	certBytes, _ := x509.CreateCertificate(rand.Reader, &template, &template, csr.PublicKey, nil)
	pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certBytes})
	resp := OnboardingResponse{
		SessionId: "sess-123",
		DeviceId:  req.DeviceInfo.DeviceId,
		Status:    "challenge_required",
		NextStep: struct {
			Action   string `json:"action"`
			Endpoint string `json:"endpoint"`
		}{
			Action:   "authenticate",
			Endpoint: "/devices/sess-123/onboard/challenge",
		},
		ExpiresAt: time.Now().Add(10 * time.Minute).Format(time.RFC3339),
	}
	json.NewEncoder(w).Encode(resp)
}

func handleChallenge(w http.ResponseWriter, r *http.Request) {
	var req ChallengeRequest
	json.NewDecoder(r.Body).Decode(&req)
	certBytes, _ := base64.StdEncoding.DecodeString(req.Response.Certificate)
	block, _ := pem.Decode(certBytes)
	cert, _ := x509.ParseCertificate(block.Bytes)
	challenge := []byte("server-challenge") // Should be random per session
	sig, _ := base64.StdEncoding.DecodeString(req.Response.PrivateKeyProof)
	err := rsa.VerifyPKCS1v15(cert.PublicKey.(*rsa.PublicKey), crypto.SHA256, challenge, sig)
	status := "passed"
	if err != nil {
		status = "failed"
	}
	resp := ChallengeResponse{
		SessionId: req.SessionId,
		Status:    status,
	}
	json.NewEncoder(w).Encode(resp)
}

func bigInt(n int64) *big.Int {
	return big.NewInt(n)
}

func main() {
	http.HandleFunc("/devices/onboard", handleOnboard)
	http.HandleFunc("/devices/sess-123/onboard/challenge", handleChallenge)
	fmt.Println("Server running on :8080")
	http.ListenAndServe(":8080", nil)
}
