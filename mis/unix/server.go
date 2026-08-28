package unix

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
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
	m.logger.Info("starting unix socket HTTP server", "socket_path", MintUnixSocketPath)

	// Clean up stale socket file
	if err := os.Remove(MintUnixSocketPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove existing socket: %v", err)
	}
	m.logger.Debug("cleaned up stale socket file", "socket_path", MintUnixSocketPath)

	listener, err := net.Listen("unix", MintUnixSocketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on unix socket %s: %v", MintUnixSocketPath, err)
	}
	defer listener.Close()
	m.logger.Info("unix socket listener created", "socket_path", MintUnixSocketPath)

	// Restrict socket permissions (owner read/write only)
	if err := os.Chmod(MintUnixSocketPath, 0o600); err != nil {
		return fmt.Errorf("failed to set socket permissions: %v", err)
	}
	m.logger.Debug("socket permissions set", "socket_path", MintUnixSocketPath, "mode", "0600")

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

	m.logger.Info(
		"server configured, beginning to serve requests",
		"read_timeout", 15*time.Second,
		"write_timeout", 15*time.Second,
		"idle_timeout", 60*time.Second,
	)

	return server.Serve(listener)
}

func (m *MintRestAPI) Stop() error {
	m.logger.Info("stopping unix socket HTTP server")

	if m.server == nil {
		m.logger.Debug("server is nil, nothing to stop")
		return nil
	}

	// Allow existing requests to finish
	ctx, cancel := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	defer cancel()

	m.logger.Info("initiating graceful shutdown", "timeout", 30*time.Second)
	err := m.server.Shutdown(ctx)
	if err == nil {
		m.logger.Info("server shut down gracefully")
		return nil
	}

	m.logger.Error("graceful shutdown failed, forcing close", "error", err)

	// Force close
	if cerr := m.server.Close(); cerr != nil {
		m.logger.Error("force close also failed", "close_error", cerr, "shutdown_error", err)
		return errors.Wrap(err, cerr.Error())
	}

	m.logger.Warn("server force-closed after graceful shutdown failure")
	return err
}

// MintX509SVIDHandler handles POST /mint/svid/x509
func (m *MintRestAPI) MintX509SVIDHandler(w http.ResponseWriter, r *http.Request) {
	m.logger.Info(
		"received request",
		"method",
		r.Method,
		"path",
		r.URL.Path,
		"remote_addr",
		r.RemoteAddr,
	)

	if r.Method != http.MethodPost {
		m.logger.Warn("method not allowed", "method", r.Method, "expected", http.MethodPost)
		helpers.WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "")
		return
	}

	// Decode request body
	var req types.MintSVIDRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		m.logger.Warn("failed to decode request body", "error", err)
		helpers.WriteError(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}
	m.logger.Debug("request body decoded successfully")

	// Validate request
	if ve := validateMintSVIDRequest(&req); ve != nil {
		m.logger.Warn("request validation failed", "error", ve)
		helpers.WriteError(w, http.StatusUnprocessableEntity, "validation failed", ve.Error())
		return
	}
	m.logger.Debug("request validation passed")

	op := operations.New(m.logger)

	// Generate X.509 SVID
	m.logger.Debug("generating X.509 SVID")
	certPEM, keyPEM, err := op.GenerateX509SVID(&req)
	if err != nil {
		m.logger.Error("failed to generate X.509 SVID", "error", err)
		helpers.WriteError(
			w,
			http.StatusInternalServerError,
			"failed to generate SVID",
			err.Error(),
		)
		return
	}
	m.logger.Info("X.509 SVID generated successfully")

	// Base64-encode PEM blocks
	resp := types.MintSVIDResponse{
		Certificate: base64.StdEncoding.EncodeToString(certPEM),
		Key:         base64.StdEncoding.EncodeToString(keyPEM),
	}

	m.logger.Debug("sending SVID response to client")
	helpers.WriteJSON(w, http.StatusOK, resp)
}
