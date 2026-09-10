package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/margo/sandbox/shared-lib/mis/trustbundle"
	"github.com/margo/sandbox/shared-lib/mis/validators"
	"go.uber.org/zap"

	"github.com/margo/sandbox/poc/device/agent/database"
)

type TrustBundleCacherIfc interface {
	Start() error // This returns first update, so that rest of the flow can start.
	Stop()
}

type TrustBundleCacher struct {
	database *database.Database
	log      *zap.SugaredLogger
	stopChan chan struct{}
	interval uint
}

func NewTrustBundleCacher(
	db *database.Database,
	interval uint,
	log *zap.SugaredLogger,
) *TrustBundleCacher {
	return &TrustBundleCacher{
		database: db,
		log:      log,
		stopChan: make(chan struct{}),
		interval: interval,
	}
}

// spiffeBundleDoc is used to partially decode a SPIFFE JWKS trust bundle
// in order to extract the optional spiffe_refresh_hint field.
type spiffeBundleDoc struct {
	// RefreshHint is the suggested polling interval in seconds, as defined by
	// the SPIFFE Bundle Endpoint Profile specification.
	RefreshHint *int `json:"spiffe_refresh_hint"`
}

// extractRefreshHint parses a raw SPIFFE JWKS bundle and returns the value of
// spiffe_refresh_hint if it is present and a positive non-zero integer.
// Returns 0 and false if the hint is absent, zero, or invalid.
func extractRefreshHint(bundle []byte) (uint, bool) {
	var doc spiffeBundleDoc
	if err := json.Unmarshal(bundle, &doc); err != nil {
		return 0, false
	}

	if doc.RefreshHint == nil || *doc.RefreshHint <= 0 {
		return 0, false
	}

	return uint(*doc.RefreshHint), true
}

// Start initialises the trust bundle getter from device settings, performs an
// immediate trust bundle fetch, and launches a background goroutine that
// refreshes the bundle on the configured (or hint-derived) interval.
//
// If device settings cannot be read or the getter cannot be constructed, Start
// logs the error and returns without launching the background goroutine.
func (tbc *TrustBundleCacher) Start() error {
	tbc.log.Infow("TrustBundleCacher starting")

	// ── Step 1: Read device settings to configure the trust bundle getter ─────

	deviceSettings, err := tbc.database.GetDeviceSettings()
	if err != nil {
		tbc.log.Errorw("Failed to fetch device settings; TrustBundleCacher will not start",
			"error", err)
		return err
	}

	miafCfg := deviceSettings.ParsedMIAF

	// ── Step 2: Construct the trust bundle getter ─────────────────────────────

	getter, err := trustbundle.New(
		miafCfg.MIS.Endpoint,               // base HTTPS URL of the MIS server
		miafCfg.MIS.CAPEM,                  // PEM-encoded CA cert for TLS verification
		miafCfg.MIS.TrustBundle.URI,        // optional well-known URI path (may be empty)
		miafCfg.MIS.TrustBundle.BundleJSON, // optional operator-supplied fallback bundle (may be nil)
		miafCfg.MIS.TrustDomain,            // SPIFFE trust domain (may be empty if not yet known)
	)
	if err != nil {
		tbc.log.Errorw("Failed to create trust bundle getter; TrustBundleCacher will not start",
			"error", err)
		return err
	}

	tbc.log.Debugw("Trust bundle getter created")

	// ── Step 3: Perform the initial trust bundle fetch ────────────────────────
	// This ensures the bundle is available immediately on startup, before the
	// first ticker tick fires.

	initialInterval, err := tbc.fetchAndStore(getter)
	if err != nil {
		return err
	}

	// ── Step 4: Start the background refresh loop ─────────────────────────────

	go tbc.refreshLoop(getter, initialInterval)
	return nil
}

// Stop signals the background refresh goroutine to exit by closing stopChan.
// It is safe to call Stop only once; closing an already-closed channel panics.
func (tbc *TrustBundleCacher) Stop() {
	tbc.log.Infow("TrustBundleCacher stopping")
	close(tbc.stopChan)
}

// refreshLoop runs the periodic trust bundle refresh ticker.
//
// It starts with initialInterval seconds and resets the ticker whenever a new
// spiffe_refresh_hint is received that differs from the current interval.
// The loop exits cleanly when stopChan is closed.
func (tbc *TrustBundleCacher) refreshLoop(getter trustbundle.Getter, initialInterval uint) {
	tickInterval := time.Duration(initialInterval) * time.Second

	tbc.log.Infow("Trust bundle refresh ticker started",
		"intervalSeconds", initialInterval)

	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-tbc.stopChan:
			// Graceful shutdown requested.
			tbc.log.Infow("Trust bundle refresh loop stopped")
			return

		case <-ticker.C:
			tbc.log.Debugw("Trust bundle refresh tick fired")

			newInterval, _ := tbc.fetchAndStore(getter)

			// Reset the ticker only when the interval has actually changed,
			// to avoid unnecessary ticker churn.
			newTickInterval := time.Duration(newInterval) * time.Second
			if newTickInterval != tickInterval {
				tbc.log.Infow("Trust bundle refresh interval updated",
					"oldIntervalSeconds", tickInterval.Seconds(),
					"newIntervalSeconds", newTickInterval.Seconds())

				ticker.Reset(newTickInterval)
				tickInterval = newTickInterval
			}
		}
	}
}

// fetchAndStore retrieves the SPIFFE trust bundle using the provided getter,
// persists it to the database, and returns the effective refresh interval in
// seconds.
//
// The returned interval is the spiffe_refresh_hint from the bundle when the
// hint is present and a positive non-zero integer; otherwise the
// TrustBundleCacher's configured interval is used as a fallback.
func (tbc *TrustBundleCacher) fetchAndStore(getter trustbundle.Getter) (uint, error) {
	ietag := tbc.database.GetTrustBundleETag()

	// Use a plain background context. Cancellation is handled externally by
	// the ticker loop via stopChan; we do not want a deadline here.
	trustDomain, bundle, etag, err := getter.GetTrustBundle(context.Background(), ietag)
	if err != nil {
		tbc.log.Errorw("Failed to retrieve trust bundle or not modified; will retry on next tick",
			"error", err)
		if errors.Is(err, trustbundle.ErrNotModified) {
			// This is not an error
			return tbc.interval, nil
		}
		// Return the currently configured interval so the ticker keeps running.
		return tbc.interval,
			fmt.Errorf("Failed to retrieve Margo SPIFFE Trust Bundle, err : %w", err)
	}

	tbc.log.Infow("Trust bundle retrieved successfully",
		"trustDomain", trustDomain,
		"etag", etag,
		"bundleSizeBytes", len(bundle))

	// Validate trust domain first,
	ok := validators.ValidateTrustDomain(trustDomain)
	if !ok {
		tbc.log.Errorw("Invalid trust domain; will retry on next tick",
			"trust domain", trustDomain)
		// Return the currently configured interval so the ticker keeps running.
		return tbc.interval, fmt.Errorf("invalid margo trust domain")
	}

	// Validate trust bundle
	err = validators.ValidateSpiffeTrustBundle(bundle, trustDomain)
	if err != nil {
		tbc.log.Errorw("Invalid Margo SPIFFE Trust Bundle; will retry on next tick",
			"error", err)
		// Return the currently configured interval so the ticker keeps running.
		return tbc.interval, fmt.Errorf("Invalid Margo SPIFFE Trust Bundle, err : %w", err)
	}

	// save in database if everything is fine
	tbc.database.SetTrustBundle(bundle, etag)
	tbc.database.SetTrustDomain(trustDomain, "")

	// ── Determine the effective refresh interval from the bundle ──────────────

	hint, ok := extractRefreshHint(bundle)
	if ok {
		tbc.log.Infow("Using spiffe_refresh_hint from trust bundle as refresh interval",
			"hintSeconds", hint)
		// Keep the cached interval in sync so Stop/restart scenarios are consistent.
		tbc.interval = hint
		return hint, nil
	}

	// spiffe_refresh_hint absent, zero, or negative — fall back to the
	// interval supplied at construction time (or last valid hint).
	tbc.log.Debugw("No valid spiffe_refresh_hint in bundle; using configured interval",
		"intervalSeconds", tbc.interval)

	return tbc.interval, nil
}
