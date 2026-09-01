package https

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	ro "github.com/margo/sandbox/mis/https/operations"
	"github.com/margo/sandbox/mis/pkg/conf"
	"github.com/margo/sandbox/mis/pkg/helpers"
	gc "github.com/margo/sandbox/mis/pkg/standard/generatedCode"
	"github.com/margo/sandbox/mis/pkg/types"
	"github.com/pkg/errors"
)

type MisRestAPI struct {
	cnf                  *conf.Config
	logger               *slog.Logger
	op                   types.MISIface
	server               *http.Server
	caCertBundleLocation string
}

func New(c *conf.Config, logger *slog.Logger) *MisRestAPI {
	return &MisRestAPI{
		cnf:    c,
		logger: logger,
		op:     ro.New(c, logger),
	}
}

func (m *MisRestAPI) Start() error {
	m.logger.Info("starting HTTPS server", "addr", m.cnf.HTTPS.Addr)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/margo", m.getDiscoveryDocument)
	mux.HandleFunc("GET /{path...}", m.getTrustBundle)
	m.logger.Debug(
		"registered HTTP routes",
		"routes",
		[]string{"GET /.well-known/margo", fmt.Sprintf("GET /%s", m.cnf.TrustBundleURI)},
	)

	m.logger.Debug("creating TLS bundle file", "cert", m.cnf.HTTPS.Cert, "ca", m.cnf.HTTPS.CA)
	bundle, err := helpers.CreateBundleFile(m.cnf.HTTPS.Cert, m.cnf.HTTPS.CA)
	if err != nil {
		m.logger.Error(
			"failed to create TLS bundle file",
			"cert",
			m.cnf.HTTPS.Cert,
			"ca",
			m.cnf.HTTPS.CA,
			"err",
			err,
		)
		return err
	}
	m.logger.Debug("TLS bundle file created successfully")

	server := &http.Server{
		Addr:    m.cnf.HTTPS.Addr,
		Handler: mux,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS13,
		},
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	m.server = server
	m.caCertBundleLocation = bundle

	m.logger.Info("HTTPS server listening", "addr", m.cnf.HTTPS.Addr, "tls_min_version", "TLS1.3")
	return server.ListenAndServeTLS(bundle, m.cnf.HTTPS.Key)
}

func (m *MisRestAPI) Stop() error {
	m.logger.Info("stopping HTTPS server")

	if m.server == nil {
		m.logger.Debug("server instance is nil, nothing to stop")
		return nil
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	defer func() {
		err := os.RemoveAll(m.caCertBundleLocation)
		if err != nil {
			m.logger.Error("failed to remove certificate bundle")
		}
	}()
	defer cancel()

	m.logger.Debug("initiating graceful shutdown", "timeout_seconds", 30)
	err := m.server.Shutdown(ctx)
	if err == nil {
		m.logger.Info("HTTPS server stopped gracefully")
		return nil
	}

	m.logger.Warn("graceful shutdown failed, forcing close", "err", err)
	if cerr := m.server.Close(); cerr != nil {
		m.logger.Error("forced close also failed", "shutdown_err", err, "close_err", cerr)
		return errors.Wrap(err, cerr.Error())
	}

	m.logger.Info("HTTPS server force-closed successfully")
	return err
}

func (m *MisRestAPI) getDiscoveryDocument(w http.ResponseWriter, r *http.Request) {
	logger := m.logger
	logger.Debug(
		"handling discovery document request",
		"remote_addr",
		r.RemoteAddr,
		"method",
		r.Method,
	)

	if ac := r.Header.Get("Accept"); ac != "application/json" {
		logger.Error("accept missing in request headers, aborting", "accept_header", ac)
		pd := gc.NewProblemDetail(
			"https://docs.margo.org/specification/problem-types#server-cannot-generate-response",
			"Server Cannot Generate Response",
			http.StatusNotAcceptable,
		).
			WithInstance("/.well-known/margo").
			WithBackoffStrategy(gc.None).
			WithDetail("accept is either missing in request headers or does not accept application/json response").
			WithRetryable(false)
		jr, _ := pd.MarshalJSON()
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusNotAcceptable)
		// #nosec G705 -- jr is json error response prepared by the appication, incorrect XSS flag
		_, err := w.Write(jr)
		if err != nil {
			logger.Error("failed to write http response", "err", err.Error())
		}
		return
	}

	logger.Debug("fetching discovery document")
	dd := m.op.GetDiscoveryDocument()
	if dd == nil {
		logger.Warn("discovery document not found")
		pd := gc.NewProblemDetail(
			"https://docs.margo.org/specification/problem-types#discovery-document-not-found",
			"Discovery Document Not Found",
			http.StatusNotFound,
		).
			WithInstance("/.well-known/margo").
			WithBackoffStrategy(gc.Exponential).
			WithRetryAfterSeconds(30).
			WithDetail("Trust domain discovery document not available.").
			WithRetryable(true)
		jr, _ := pd.MarshalJSON()
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusNotFound)
		// #nosec G705 -- jr is json error response prepared by the appication, incorrect XSS flag
		_, err := w.Write(jr)
		if err != nil {
			logger.Error("failed to write http response", "err", err.Error())
		}
		return
	}

	rawResp, err := dd.MarshalJSON()
	if err != nil {
		logger.Error("failed to marshal discovery document", "err", err.Error())
		pd := gc.NewProblemDetail(
			"https://docs.margo.org/specification/problem-types#discovery-document-not-found",
			"Discovery Document Not Found",
			http.StatusNotFound,
		).
			WithInstance("/.well-known/margo").
			WithBackoffStrategy(gc.Exponential).
			WithRetryAfterSeconds(30).
			WithDetail(fmt.Sprintf("failed to marshal discovery document in json format, err: %s", err.Error())).
			WithRetryable(true)

		jr, _ := pd.MarshalJSON()
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusNotFound)
		// #nosec G705 -- jr is json error response prepared by the appication, incorrect XSS flag
		_, err := w.Write(jr)
		if err != nil {
			logger.Error("failed to write http response", "err", err.Error())
		}
		return
	}

	hash := sha256.Sum256(rawResp)
	etag := fmt.Sprintf("\"%s\"", hex.EncodeToString(hash[:]))
	logger.Debug("computed ETag for discovery document", "etag", etag)

	if r.Header.Get("If-None-Match") == etag {
		logger.Info("etag matched, cached copy still valid")
		w.WriteHeader(http.StatusNotModified)
		return
	}

	logger.Info("serving discovery document", "etag", etag, "size_bytes", len(rawResp))
	w.Header().Set("ETag", etag)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(rawResp)
	if err != nil {
		logger.Error("failed to write http response", "err", err.Error())
	}
}

func (m *MisRestAPI) getTrustBundle(w http.ResponseWriter, r *http.Request) {
	logger := m.logger
	path := r.PathValue("path")
	logger.Debug("handling trust bundle request", "remote_addr", r.RemoteAddr, "path", path)

	if path != m.cnf.TrustBundleURI {
		logger.Error("trust bundle uri did not match",
			"expected", m.cnf.TrustBundleURI,
			"value", path)
		pd := gc.NewProblemDetail(
			"about:blank",
			"Resource Not Found",
			http.StatusNotFound,
		).
			WithInstance(path).
			WithBackoffStrategy(gc.None).
			WithDetail(fmt.Sprintf("path %s not found", path)).
			WithRetryable(false)

		jr, _ := pd.MarshalJSON()
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusNotFound)
		// #nosec G705 -- jr is json error response prepared by the appication, incorrect XSS flag
		_, err := w.Write(jr)
		if err != nil {
			logger.Error("failed to write http response", "err", err.Error())
		}
		return
	}

	logger.Debug("fetching trust bundle from operations")
	tb, err := m.op.GetTrustBundle()
	if err != nil {
		logger.Error("failed to get trust bundle", "err", err.Error())
		pd := gc.NewProblemDetail(
			"https://docs.margo.org/specification/problem-types#spiffe-bundle-not-found",
			"Bundle Not Found",
			http.StatusNotFound,
		).
			WithInstance(path).
			WithBackoffStrategy(gc.Exponential).
			WithDetail(fmt.Sprintf("SPIFFE bundle unavailable, err: %s", err.Error())).
			WithRetryable(true).
			WithRetryAfterSeconds(30)

		jr, _ := pd.MarshalJSON()
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusNotFound)
		// #nosec G705 -- jr is json error response prepared by the appication, incorrect XSS flag
		_, err := w.Write(jr)
		if err != nil {
			logger.Error("failed to write http response", "err", err.Error())
		}
		return
	}

	rawBundle, err := tb.Marshal()
	if err != nil {
		logger.Error("failed to marshal trust bundle", "err", err.Error())
		pd := gc.NewProblemDetail(
			"https://docs.margo.org/specification/problem-types#spiffe-bundle-not-found",
			"Bundle Not Found",
			http.StatusNotFound,
		).
			WithInstance(path).
			WithBackoffStrategy(gc.None).
			WithDetail(fmt.Sprintf("SPIFFE bundle unavailable, failed to marshal; err: %s", err.Error())).
			WithRetryable(false)

		jr, _ := pd.MarshalJSON()
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusNotFound)
		// #nosec G705 -- jr is json error response prepared by the appication, incorrect XSS flag
		_, err := w.Write(jr)
		if err != nil {
			logger.Error("failed to write http response", "err", err.Error())
		}
		return
	}

	hash := sha256.Sum256(rawBundle)
	etag := fmt.Sprintf("\"%s\"", hex.EncodeToString(hash[:]))
	logger.Debug("computed ETag for trust bundle", "etag", etag)

	if r.Header.Get("If-None-Match") == etag {
		logger.Info("etag matched, cached copy still valid")
		w.WriteHeader(http.StatusNotModified)
		return
	}

	logger.Info("serving trust bundle", "path", path, "etag", etag, "size_bytes", len(rawBundle))
	w.Header().Set("ETag", etag)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(rawBundle)
	if err != nil {
		logger.Error("failed to write http response", "err", err.Error())
	}
}
