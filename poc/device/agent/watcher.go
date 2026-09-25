package main

import (
	"encoding/json"

	"github.com/margo/sandbox/poc/device/agent/database"
	"github.com/margo/sandbox/shared-lib/mis/validators"
	"github.com/margo/sandbox/shared-lib/watcher"
	"go.uber.org/zap"
)

type AuthFileWatcherIfc interface {
	Start() error // This returns first update, so that rest of the flow can start.
	Stop()
}

type AuthFileWatcher struct {
	authFilePath string
	database     *database.Database
	log          *zap.SugaredLogger
	cancelFunc   watcher.CancelFunc
}

func NewAuthFileWatcher(
	authFilePath string,
	db *database.Database,
	log *zap.SugaredLogger,
) *AuthFileWatcher {
	return &AuthFileWatcher{
		authFilePath: authFilePath,
		database:     db,
		log:          log,
	}
}

func (afw *AuthFileWatcher) Start() error {
	cancelFunc, changes, err := watcher.New(afw.authFilePath,
		func(data []byte) ([]string, error) {
			result := make([]string, 0)
			err := json.Unmarshal(data, &result)
			return result, err
		}, 10)
	if err != nil {
		return err
	}
	afw.cancelFunc = cancelFunc

	go func() {
		for v := range changes {
			// Validate Authorized spiffe Ids here
			validated := true
			for _, spid := range v {
				// Authorization list for device-agent will contain SPIFFE IDs of WFMs, hence using principal WFM here.
				err := validators.ValidateSpiffeID(spid, validators.PrincipalWFM)
				if err == nil {
					continue
				}
				afw.log.Errorw(
					"failed to update authorized wfm spiffeIds, spiffe id is invalid",
					"spiffeId",
					spid,
					"err",
					err.Error(),
				)
				validated = false
			}

			if !validated {
				afw.log.Warn(
					"authorized wfm spiffeId updation failed due to validation. will keep using older list",
				)
				continue
			}
			afw.database.SetAuthorizedWFMs(v)
			afw.log.Info("authorized wfm spiffeId updation successful")
		}
	}()

	return nil
}

func (afw *AuthFileWatcher) Stop() {
	afw.cancelFunc()
}
