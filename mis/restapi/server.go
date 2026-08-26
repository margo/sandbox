package discovery

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/margo/sandbox/mis/pkg/conf"
	"github.com/margo/sandbox/mis/pkg/helpers"
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
		w.WriteHeader(http.StatusBadRequest)
		logger.Error("accept missing in request headers, aborting")
		// TODO: What should be the error here?
		return
	}

	dd := m.op.GetDiscoveryDocument()
	if dd == nil {
		logger.Warn("discovery document not found")
		w.WriteHeader(http.StatusNotFound)
		return
	}
	rawResp, err := json.Marshal(*dd)
	if err != nil {
		logger.Error("failed to marshal discovery document", "err", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		// TODO: What should be the error here? Should we use not found?
		return
	}

	// hash := sha256.Sum256(rawResp)
	// etag := fmt.Sprintf("\"%s\"", hex.EncodeToString(hash[:]))

	// if r.Header.Get("If-None-Match") == etag {
	// 	logger.Info("etag matched, cached copy still valid")
	// 	w.WriteHeader(http.StatusNotModified)
	// 	return
	// }
	// w.Header().Set("ETag", etag)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(rawResp)
}

func (m *MisRestAPI) getTrustBundle(w http.ResponseWriter, r *http.Request) {
	// Verify trust bundle here
	logger := m.logger
	path := r.PathValue("path")
	if path != m.cnf.TrustBundleURI {
		logger.Error("trust bundle uri did not match",
			"expected",
			m.cnf.TrustBundleURI,
			"value",
			path)
		// TODO: Should we use some other error here?
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// TODO: Complete this
}
