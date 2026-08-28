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
}

func New(cnf *conf.Config) types.MISIface {
	return &Operation{
		trustDomain: cnf.TrustDomain,
	}
}
