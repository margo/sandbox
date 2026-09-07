// agent.go - Updated version
package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/margo/sandbox/poc/device/agent/database"
	"github.com/margo/sandbox/poc/device/agent/types"
	wfm "github.com/margo/sandbox/poc/wfm/cli"
	"github.com/margo/sandbox/shared-lib/crypto"
	"github.com/margo/sandbox/shared-lib/workloads"
	"github.com/margo/sandbox/standard/generatedCode/wfm/sbi"
	"go.uber.org/zap"
)

// 1. Device onboarding on wfm
// 2. Device capabilities reporting to the wfm
// 3. State seeking/syncing with wfm
// 4. Deployment status updates to the wfm
type Agent struct {
	log            *zap.SugaredLogger
	auth           *DeviceClientSettings
	capabilities   *sbi.DeviceCapabilitiesManifest
	config         types.Config
	database       database.DatabaseIfc
	syncer         StateSyncerIfc
	deployer       DeploymentManagerIfc
	monitor        DeploymentMonitorIfc
	statusReporter StatusReporterIfc
}

func NewAgent(configPath string) (*Agent, error) {
	logger, _ := zap.NewDevelopment()
	log := logger.Sugar()

	// Load configuration
	cfg, err := types.LoadConfig(configPath)
	if err != nil {
		return nil, err
	}
	log.Debugw("printing database location from config", "dbDir", cfg.Database.DataDir)

	// Create database
	db := database.NewDatabase(cfg.Database.DataDir)

	// Prepare request editors (e.g., request signer) for WFM client
	clientOptions := []wfm.HTTPApiClientOptions{}

	// Create WFM client using configured URL
	wfmUrl := cfg.Wfm.SbiURL

	clientOptions = append(clientOptions, sbi.WithRequestEditorFn(PreflightLogger(100, log)))

	// TODO: MIAF SUP (PR2) — RFC 9421 HTTP Message Signatures (PR1) are replaced by mTLS.
	// This entire RequestSigner plugin block should be removed when MIAF is implemented.
	// Replace with: mTLS client certificate (X.509-SVID) configured in tls.Config.
	// See: shared-lib/crypto/signer.go — marked for deletion on MIAF implementation.

	// clientOptions = append(clientOptions, sbi.WithRequestEditorFn(signer.SignRequest))

	wfmClient, err := wfm.NewSbiHTTPClient(wfmUrl, clientOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to create WFM client: %w", err)
	}

	opts := []Option{}
	var helmClient *workloads.HelmClient
	var composeClient *workloads.DockerComposeCliClient
	for _, runtime := range cfg.Runtimes {
		if runtime.Kubernetes != nil {
			// Create Helm client
			helmClient, err = workloads.NewHelmClient(runtime.Kubernetes.KubeconfigPath)
			if err != nil {
				return nil, err
			}
			opts = append(opts, WithEnableHelmDeployment())
		}

		if runtime.Docker != nil {
			// Create docker compose client
			composeClient, err = workloads.NewDockerComposeCliClient(
				workloads.DockerConnectivityParams{
					ViaSocket: &workloads.DockerConnectionViaSocket{
						SocketPath: runtime.Docker.Url,
					},
				},
				"data/composeFiles",
			)
			if err != nil {
				return nil, err
			}
			opts = append(opts, WithEnableComposeDeployment())
		}
	}
	if helmClient == nil && composeClient == nil {
		return nil, fmt.Errorf(
			"neither kubernetes nor docker runtime objects were able to be attached, please check info if you have misplaced their settings",
		)
	}

	var deviceSettings *DeviceClientSettings
	deviceSettings, err = NewDeviceSettings(wfmClient, db, log, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize device settings: %w", err)
	}

	log.Infow(
		"Device details",
		"deviceId", deviceSettings.deviceClientId,
		// "hasValidDeviceCertificate", hasValidDeviceCertificate, // Uncomment when MIAF Related stuff is implemented
		// TODO: MIAF SUP — hasRequestSigningKey rename to "hasMTLSClientCert" when RFC 9421 removed
		// "canSignRequests", hasRequestSigningKey, // Uncomment when MIAF related stuff is implemented
		"supportedDeploymentTypes", deviceSettings.supportedDeploymentTypes,
		"supportedRuntimes", deviceSettings.supportedRuntimes,
	)

	capabilities, err := types.LoadCapabilities(cfg.Capabilities.ReadFromFile)
	if err != nil {
		log.Errorw(
			"failed to load the capabilities file, please resolve the issue as the capabilities will not be reported until next restart",
			"err",
			err.Error(),
		)
	}

	// Create components
	deployer := NewDeploymentManager(db, capabilities, helmClient, composeClient, log)
	monitor := NewDeploymentMonitor(db, helmClient, composeClient, log)
	syncer := NewStateSyncer(
		db,
		wfmClient,
		deviceSettings.deviceClientId,
		cfg.StateSeeking.Interval,
		log,
	)
	statusReporter := NewStatusReporter(db, wfmClient, deviceSettings.deviceClientId, log)

	return &Agent{
		database:       db,
		syncer:         syncer,
		deployer:       deployer,
		monitor:        monitor,
		auth:           deviceSettings,
		statusReporter: statusReporter,
		log:            log,
		config:         *cfg,
		capabilities:   capabilities,
	}, nil
}

func (a *Agent) Start() error {
	a.log.Info("Starting Workload Fleet Management Client")

	var deviceId string
	var err error

	// 1. Onboard device
	deviceSettings, err := a.database.GetDeviceSettings()
	if err != nil {
		return err
	}
	deviceId = deviceSettings.DeviceClientId

	// 2. Report capabilities

	a.capabilities.Properties.Id = deviceId
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := a.auth.ReportCapabilities(ctx, *a.capabilities); err != nil {
		a.log.Errorw("failed to report the capabilities, ", "err", err.Error())
	}

	// 3. Start all components
	a.statusReporter.Start()
	a.deployer.Start()
	a.monitor.Start()
	a.syncer.Start()

	a.log.Infow(
		"Workload Fleet Management Client started successfully",
		"capabilitiesFile", a.config.Capabilities.ReadFromFile,
		"stateSeekingInterval", a.config.StateSeeking.Interval,
		"sbiUrl", a.config.Wfm.SbiURL,
	)
	return nil
}

func (a *Agent) Stop() error {
	a.log.Info("Stopping Workload Fleet Management Client")

	a.syncer.Stop()
	a.deployer.Stop()
	a.monitor.Stop()
	a.statusReporter.Stop()
	a.database.TriggerDataPersist()

	a.log.Info("Workload Fleet Management Client stopped")
	return nil
}

func main() {
	// Define command-line flags
	configPath := flag.String(
		"config",
		"poc/device/agent/config/config.yaml", // default value
		"Path to the YAML configuration file for the Margo device agent",
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nMargo Device Workload Fleet Management Client\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if configPath == nil {
		log.Fatal("--config is mandatory command line argument")
	}

	agent, err := NewAgent(*configPath)
	if err != nil {
		log.Fatal(err)
	}

	if err := agent.Start(); err != nil {
		log.Fatal(err)
	}

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	if err := agent.Stop(); err != nil {
		log.Println("error occured while stopping the agent", err)
	}
	log.Println("agent stopped successfully!")
}

// PreflightLogger returns a RequestEditorFn that logs method, URL, headers (redacted)
// and a truncated preview of the request body. It restores req.Body so the request
// remains intact for other editors (e.g. signing) and for sending.
func PreflightLogger(
	maxPreviewBytes int,
	logger *zap.SugaredLogger,
) func(ctx context.Context, req *http.Request) error {
	// headers we always redact
	allow := map[string]struct{}{
		"content-type":     {},
		"content-length":   {},
		"accept":           {},
		"accept-encoding":  {},
		"user-agent":       {},
		"x-request-id":     {},
		"x-correlation-id": {},
		"if-none-match":    {},
	}

	isTextLike := func(ct string) bool {
		ct = strings.ToLower(ct)
		if strings.Contains(ct, "json") || strings.Contains(ct, "xml") ||
			strings.HasPrefix(ct, "text/") {
			return true
		}
		return false
	}

	return func(ctx context.Context, req *http.Request) error {
		// start-line
		method := req.Method
		urlStr := ""
		if req.URL != nil {
			sanitized := *req.URL
			if sanitized.RawQuery != "" {
				sanitized.RawQuery = "[REDACTED]"
			}
			urlStr = sanitized.String()
		}

		headers := make(map[string][]string, len(req.Header))
		for k, vv := range req.Header {
			if _, ok := allow[strings.ToLower(k)]; ok {
				headers[k] = vv
			} else {
				headers[k] = []string{"[REDACTED]"}
			}
		}

		// body preview logic:
		var preview string
		var truncated bool
		var bodyLen int64 = -1

		// If request has a GetBody factory, use it to sample body without consuming original
		if req.GetBody != nil {
			r, err := req.GetBody()
			if err == nil {
				defer r.Close()
				// read up to maxPreviewBytes+1 to detect truncation
				limited := io.LimitReader(r, int64(maxPreviewBytes)+1)
				b, _ := io.ReadAll(limited)
				bodyLen = int64(len(b))
				if len(b) > maxPreviewBytes {
					truncated = true
					b = b[:maxPreviewBytes]
				}
				if isTextLike(req.Header.Get("Content-Type")) {
					preview = string(b)
				} else {
					// try to detect small textual content, else base64
					if http.DetectContentType(b)[:4] == "text" ||
						strings.Contains(strings.ToLower(req.Header.Get("Content-Type")), "json") {
						preview = string(b)
					} else {
						preview = base64.StdEncoding.EncodeToString(b)
					}
				}
			}
		} else if req.Body != nil {
			// If ContentLength provided and very large, skip body capture to avoid OOM
			if req.ContentLength > 0 && req.ContentLength > int64(maxPreviewBytes*10) {
				// skip capturing large bodies when GetBody not available
				preview = "<body too large to capture; no GetBody available>"
			} else {
				// read full body into memory (be cautious here in production)
				bodyBytes, err := io.ReadAll(req.Body)
				if err != nil {
					preview = "<error reading body>"
					// Restore a closed body to prevent downstream failures (best-effort)
					req.Body = io.NopCloser(bytes.NewReader(nil))
				} else {
					bodyLen = int64(len(bodyBytes))
					// restore body for downstream editors/sender
					req.Body = io.NopCloser(bytes.NewReader(bodyBytes))

					// build preview (truncate if needed)
					b := bodyBytes
					if len(b) > maxPreviewBytes {
						truncated = true
						b = b[:maxPreviewBytes]
					}
					if isTextLike(req.Header.Get("Content-Type")) {
						preview = string(b)
					} else {
						// detect if it's actually textual
						detected := http.DetectContentType(b)
						if strings.HasPrefix(detected, "text/") ||
							strings.Contains(
								strings.ToLower(req.Header.Get("Content-Type")),
								"json",
							) {
							preview = string(b)
						} else {
							preview = base64.StdEncoding.EncodeToString(b)
						}
					}
				}
			}
		} else {
			preview = "<no body>"
			bodyLen = 0
		}

		// Log structured info
		fields := []interface{}{
			"method", method,
			"url", urlStr,
			"headers", headers,
			"body_preview", preview,
			"body_truncated", truncated,
			"body_len", bodyLen,
		}
		logger.Infow("Preflight-http-request", fields...)
		return nil
	}
}

// TODO: Move these functions so that they can be reused for calling MIS service
// pass caPath if you want to use some particular ca to verify the certificates
func TLSVerifier(caPath *string) wfm.HTTPApiClientOptions {
	// TODO: we should instead create our own http client and then set that into the openapi client
	// the current way is a slightly longer route to acheive things
	return func(client *sbi.Client) error {
		// Validate client
		if client == nil {
			return fmt.Errorf("client cannot be nil")
		}

		// Create TLS config
		tlsConfig := &tls.Config{}

		// Load and configure custom CA if provided
		if caPath != nil && *caPath != "" {
			var err error
			tlsConfig, err = crypto.LoadCustomCA(*caPath)
			if err != nil {
				return err
			}
		}

		// Configure HTTP client with TLS
		return configureClientTLS(client, tlsConfig)
	}
}

// Helper function to configure client TLS
func configureClientTLS(client *sbi.Client, tlsConfig *tls.Config) error {
	httpClient, ok := client.Client.(*http.Client)
	if !ok {
		return fmt.Errorf("client.Client is not *http.Client, cannot configure TLS")
	}

	// Get or create transport
	var transport *http.Transport
	if httpClient.Transport != nil {
		if existingTransport, ok := httpClient.Transport.(*http.Transport); ok {
			transport = existingTransport.Clone()
		} else {
			transport = &http.Transport{}
		}
	} else {
		transport = &http.Transport{}
	}

	// Configure TLS
	transport.TLSClientConfig = tlsConfig
	httpClient.Transport = transport

	return nil
}
