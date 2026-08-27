package restapi

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/margo/sandbox/mis/pkg/conf"
	"github.com/margo/sandbox/mis/pkg/helpers"
	gc "github.com/margo/sandbox/mis/pkg/standard/generatedCode"
	"github.com/margo/sandbox/mis/pkg/types"
	ro "github.com/margo/sandbox/mis/restapi/operations"
)

type MisRestAPI struct {
	cnf    *conf.Config
	logger *slog.Logger
	op     types.MISIface
}

func New(c *conf.Config, logger *slog.Logger) *MisRestAPI {
	return &MisRestAPI{
		cnf:    c,
		logger: logger,
		op:     ro.New(c),
	}
}

func (m *MisRestAPI) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/margo", m.getDiscoveryDocument)
	mux.HandleFunc("GET /{path}", m.getTrustBundle)
	bundle, err := helpers.CreateBundleFile(m.cnf.HTTPS.Cert, m.cnf.HTTPS.CA)
	if err != nil {
		return err
	}
	return http.ListenAndServeTLS(m.cnf.HTTPS.Addr, bundle, m.cnf.HTTPS.Key, mux)
}

func (m *MisRestAPI) getDiscoveryDocument(w http.ResponseWriter, r *http.Request) {
	logger := m.logger
	if ac := r.Header.Get("Accept"); ac != "application/json" {
		logger.Error("accept missing in request headers, aborting")
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
		w.Write(jr)
		w.WriteHeader(http.StatusNotAcceptable)
		return
	}

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
		w.Write(jr)
		w.WriteHeader(http.StatusNotFound)
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
		w.Write(jr)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	hash := sha256.Sum256(rawResp)
	etag := fmt.Sprintf("\"%s\"", hex.EncodeToString(hash[:]))

	if r.Header.Get("If-None-Match") == etag {
		logger.Info("etag matched, cached copy still valid")
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Set("ETag", etag)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(rawResp)
}

func (m *MisRestAPI) getTrustBundle(w http.ResponseWriter, r *http.Request) {
	// Verify trust bundle here
	logger := m.logger
	path := r.PathValue("path")
	// If user is sending TrustBundleURI different from what we are advertising, return 404
	if path != m.cnf.TrustBundleURI {
		logger.Error("trust bundle uri did not match",
			"expected",
			m.cnf.TrustBundleURI,
			"value",
			path)
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
		w.Write(jr)
		w.WriteHeader(http.StatusNotFound)
		return
	}

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
		w.Write(jr)
		w.WriteHeader(http.StatusNotFound)
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
		w.Write(jr)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	hash := sha256.Sum256(rawBundle)
	etag := fmt.Sprintf("\"%s\"", hex.EncodeToString(hash[:]))

	if r.Header.Get("If-None-Match") == etag {
		logger.Info("etag matched, cached copy still valid")
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("ETag", etag)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(rawBundle)
}
