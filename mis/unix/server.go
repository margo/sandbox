package unix

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/margo/sandbox/mis/pkg/conf"
	"github.com/margo/sandbox/mis/pkg/helpers"
	"github.com/margo/sandbox/mis/pkg/types"
	"github.com/margo/sandbox/mis/unix/operations"
)

const (
	socketPath      = "/tmp/mint.sock"
	shutdownTimeout = 10 * time.Second
)

type MintRestAPI struct {
	cnf    *conf.Config
	logger *slog.Logger
}

func New(c *conf.Config, logger *slog.Logger) *MintRestAPI {
	return &MintRestAPI{
		cnf:    c,
		logger: logger,
	}
}

// Starts the unix socket HTTP server for CLI client to connect to & mint certificates
func (m *MintRestAPI) Start() error {
	// Clean up stale socket file
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		log.Fatalf("failed to remove existing socket: %v", err)
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatalf("failed to listen on unix socket %s: %v", socketPath, err)
	}
	defer listener.Close()

	// Restrict socket permissions (owner read/write only)
	if err := os.Chmod(socketPath, 0o600); err != nil {
		log.Fatalf("failed to set socket permissions: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /mint/svid/x509", m.MintX509SVIDHandler)

	server := &http.Server{
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return server.Serve(listener)
}

// MintX509SVIDHandler handles POST /mint/svid/x509
func (m *MintRestAPI) MintX509SVIDHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		helpers.WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "")
		return
	}

	// Decode request body
	var req types.MintSVIDRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		helpers.WriteError(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	// Validate request
	if ve := validateMintSVIDRequest(&req); ve != nil {
		helpers.WriteError(w, http.StatusUnprocessableEntity, "validation failed", ve.Error())
		return
	}

	op := operations.New()

	// Generate X.509 SVID
	certPEM, keyPEM, err := op.GenerateX509SVID(&req)
	if err != nil {
		helpers.WriteError(
			w,
			http.StatusInternalServerError,
			"failed to generate SVID",
			err.Error(),
		)
		return
	}

	// Base64-encode PEM blocks
	resp := types.MintSVIDResponse{
		Certificate: base64.StdEncoding.EncodeToString(certPEM),
		Key:         base64.StdEncoding.EncodeToString(keyPEM),
	}

	helpers.WriteJSON(w, http.StatusOK, resp)
}

// package main

// import (
// 	"context"
// 	"log"
// 	"net"
// 	"net/http"
// 	"os"
// 	"os/signal"
// 	"syscall"
// 	"time"

// 	"yourmodule/handlers"
// )

// func main() {

// 	// Graceful shutdown
// 	quit := make(chan os.Signal, 1)
// 	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

// 	go func() {
// 		log.Printf("Server listening on unix://%s", socketPath)
// 		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
// 			log.Fatalf("server error: %v", err)
// 		}
// 	}()

// 	<-quit
// 	log.Println("Shutting down server...")

// 	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
// 	defer cancel()

// 	if err := server.Shutdown(ctx); err != nil {
// 		log.Fatalf("forced shutdown: %v", err)
// 	}
// 	log.Println("Server stopped")
// }
