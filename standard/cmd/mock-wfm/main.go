package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/margo/sandbox/standard/cmd/mock-wfm/conformance_ogen/api"
)

func main() {
	// Create mock server instance
	handler := NewMockWFMServer()

	// Create the ogen server with your handler and path prefix
	// Symphony base path: /v1alpha2/margo
	server, err := api.NewServer(handler, api.WithPathPrefix("/v1alpha2/margo"))
	if err != nil {
		log.Fatalf("❌ Failed to create server: %v", err)
	}

	// Define listen addresses
	httpAddr := "0.0.0.0:8090"
	httpsAddr := "0.0.0.0:9090"

	fmt.Printf("🚀 Mock WFM Server starting...\n")
	fmt.Printf("📍 HTTP Base URL:  http://localhost:8090/v1alpha2/margo\n")
	fmt.Printf("📍 HTTPS Base URL: https://localhost:9090/v1alpha2/margo\n")
	fmt.Println("")
	fmt.Println("📖 Available API Endpoints:")
	fmt.Println("")
	fmt.Println("Device Capabilities:")
	fmt.Println("  POST /api/v1/clients/{clientId}/capabilities - Report device capabilities")
	fmt.Println("  PUT  /api/v1/clients/{clientId}/capabilities - Update device capabilities")
	fmt.Println("")
	fmt.Println("Deployments:")
	fmt.Println("  GET  /api/v1/clients/{clientId}/deployments             - List deployments")
	fmt.Println("  GET  /api/v1/clients/{clientId}/deployments/{deploymentId}/{digest} - Get deployment")
	fmt.Println("  POST /api/v1/clients/{clientId}/deployments/{deploymentId}/status   - Report status")
	fmt.Println("")
	fmt.Println("Bundles:")
	fmt.Println("  GET  /api/v1/clients/{clientId}/bundles/{digest} - Get bundle")
	fmt.Println("")
	fmt.Println("Onboarding:")
	fmt.Println("  GET  /api/v1/onboarding/certificate - Download Root CA certificate")
	fmt.Println("  POST /api/v1/onboarding             - Complete onboarding")
	fmt.Println("")

	// Print registered devices every 30 seconds (for demo/debugging)
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			fmt.Println("")
			fmt.Println("📊 Currently registered devices:")
			handler.ListDevices()
			fmt.Println("📊 Deployment statuses:")
			handler.ListDeploymentStatuses()
			fmt.Println("")
		}
	}()

	// Start HTTP server in goroutine
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		fmt.Printf("🌐 HTTP server listening on http://%s\n", httpAddr)
		debugServer := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(os.Stderr, "[DEBUG] HTTP request: %s %s\n", r.Method, r.RequestURI)
			server.ServeHTTP(w, r)
		})
		if err := http.ListenAndServe(httpAddr, debugServer); err != nil && err != http.ErrServerClosed {
			log.Printf("❌ HTTP Server error: %v", err)
		}
	}()

	// Start HTTPS server in goroutine
	go func() {
		defer wg.Done()
		certFile, keyFile := getCertificatePaths()
		if certFile == "" || keyFile == "" {
			fmt.Println("⚠️  HTTPS certificates not found, HTTPS server will not start")
			fmt.Println("   Generate certificates with: bash generate-certs.sh .")
			return
		}

		// Check if certificate files exist
		if _, err := os.Stat(certFile); os.IsNotExist(err) {
			fmt.Printf("⚠️  Certificate file not found: %s\n", certFile)
			return
		}
		if _, err := os.Stat(keyFile); os.IsNotExist(err) {
			fmt.Printf("⚠️  Key file not found: %s\n", keyFile)
			return
		}

		fmt.Printf("🔐 HTTPS server listening on https://%s\n", httpsAddr)
		fmt.Printf("   Using certificate: %s\n", certFile)
		fmt.Printf("   Using key file: %s\n", keyFile)
		debugServer := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(os.Stderr, "[DEBUG] HTTPS request: %s %s\n", r.Method, r.RequestURI)
			fmt.Fprintf(os.Stderr, "[DEBUG]   Path: %s, URL: %v\n", r.URL.Path, r.URL)
			server.ServeHTTP(w, r)
		})
		if err := http.ListenAndServeTLS(httpsAddr, certFile, keyFile, debugServer); err != nil && err != http.ErrServerClosed {
			log.Printf("❌ HTTPS Server error: %v", err)
		}
	}()

	fmt.Println("✅ Ready to accept requests...")
	fmt.Println("")

	wg.Wait()
}

// getCertificatePaths returns the paths to the server certificate and key files
func getCertificatePaths() (certFile, keyFile string) {
	// Get the directory where the binary is running
	exePath, err := os.Executable()
	if err != nil {
		return "", ""
	}
	exeDir := filepath.Dir(exePath)

	// Try to find certificates in the same directory as the binary
	certPath := filepath.Join(exeDir, "server-cert.pem")
	keyPath := filepath.Join(exeDir, "server-key.pem")

	// Check if files exist
	if _, err := os.Stat(certPath); err == nil {
		if _, err := os.Stat(keyPath); err == nil {
			return certPath, keyPath
		}
	}

	return "", ""
}
