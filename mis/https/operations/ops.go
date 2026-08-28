package operations

import (
	"log/slog"

	"github.com/margo/sandbox/mis/pkg/conf"
	"github.com/margo/sandbox/mis/pkg/types"
)

type Operation struct {
	trustDomain    string
	trustBundleURI string
	ca             conf.CAConfig
	logger         *slog.Logger
	addr           string
}

func New(cnf *conf.Config, logger *slog.Logger) types.MISIface {
	return &Operation{
		trustDomain:    cnf.TrustDomain,
		trustBundleURI: cnf.TrustBundleURI,
		ca:             *cnf.CA,
		logger:         logger,
		addr:           cnf.HTTPS.Addr,
	}
}
