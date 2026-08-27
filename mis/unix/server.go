package unix

import (
	"context"
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
	"github.com/pkg/errors"
)

const (
	MintUnixSocketPath = "/tmp/mint.sock"
)

type MintRestAPI struct {
	cnf    *conf.Config
	logger *slog.Logger
	server *http.Server
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
	if err := os.Remove(MintUnixSocketPath); err != nil && !os.IsNotExist(err) {
		log.Fatalf("failed to remove existing socket: %v", err)
	}

	listener, err := net.Listen("unix", MintUnixSocketPath)
	if err != nil {
		log.Fatalf("failed to listen on unix socket %s: %v", MintUnixSocketPath, err)
	}
	defer listener.Close()

	// Restrict socket permissions (owner read/write only)
	if err := os.Chmod(MintUnixSocketPath, 0o600); err != nil {
		log.Fatalf("failed to set socket permissions: %v", err)
	}

	// Non normative endpoints go here
	mux := http.NewServeMux()
	mux.HandleFunc("POST /mint/svid/x509", m.MintX509SVIDHandler)

	server := &http.Server{
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	m.server = server

	return server.Serve(listener)
}

func (m *MintRestAPI) Stop() error {
	if m.server == nil {
		return nil
	}
	// Allow existing requests to finish56
	ctx, cancel := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	defer cancel()

	err := m.server.Shutdown(ctx)
	if err == nil {
		return nil
	}

	// Force close
	if cerr := m.server.Close(); cerr != nil {
		return errors.Wrap(err, cerr.Error())
	}
	return err
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
