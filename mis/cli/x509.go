package cli

// cli/x509.go
// Defines the "x509" subcommand under "mint" for generating X.509 SVIDs.
// Handles argument parsing, validation, and delegates to the integration layer.

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"log"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/margo/sandbox/mis/pkg/types"
	"github.com/margo/sandbox/mis/unix"
	"github.com/margo/sandbox/mis/unix/client"
	"github.com/spf13/cobra"
)

const (
	// defaultTTL is the default time-to-live for a generated X.509 SVID (24 hours in seconds).
	defaultTTL = 24 * 60 * 60 // 86400 seconds

	// spiffeScheme is the required URI scheme for a valid SPIFFE ID.
	spiffeScheme = "spiffe"

	CertName    string = "payload-cert.pem"
	CertKeyName string = "payload-key.pem"
)

// x509Flags holds all parsed flag values for the x509 subcommand.
type x509Flags struct {
	// DNSNames is a list of DNS SANs to include in the X.509 SVID.
	// Can be specified multiple times: --dns foo.example.com --dns bar.example.com
	DNSNames []string

	// SpiffeID is the SPIFFE ID to embed in the SVID.
	// Must follow the format: spiffe://<trust-domain>/<path>
	SpiffeID string

	// Path on disk where CSR is present, for generating SVID
	// Should contain a valid SPIFFE ID in URI SAN
	CSRPath string

	// TTL is the validity duration in seconds for the generated SVID.
	// Defaults to 86400 (24 hours) if not provided.
	TTL int

	// OutputDir is the directory where the generated SVID and private key will be saved.
	OutputDir string
}

// flags holds the parsed values for the x509 subcommand flags.
var flags x509Flags

// x509Cmd represents the "mint x509" subcommand.
// It mints an X.509 SVID with the specified parameters.
var x509Cmd = &cobra.Command{
	Use:   "x509",
	Short: "Mint an X.509 SVID",
	Long: `Mint an X.509 SPIFFE Verifiable Identity Document (SVID).

The generated SVID and its corresponding private key will be saved
to the specified output directory (defaults to the current working directory).

Output files (will be overwritten if they already exist):
  payload-cert.pem  — the generated X.509 SVID certificate
  payload-key.pem   — the corresponding private key (If CSR is not provided)

Required Flags:
  --spiffeID   The SPIFFE ID to embed in the SVID.
			   Must follow the format: spiffe://<trust-domain>/<path>.
			   This is optional only when CSR is provided. 
  --csrPath    Path on disk of CSR for generating SVID.
			   Should contain 1 SPIFFE ID in URI SAN
			   spiffe Id format: spiffe://<trust-domain>/<path>.
			   If not provided, spiffeID is required, and key is returned as well. 
				

Optional Flags:
  --dns        DNS Subject Alternative Name (SAN) to include in the SVID.
			   Can be specified multiple times for multiple DNS names.
  --ttl        Time-to-live in seconds for the SVID. Defaults to 86400 (24 hours).
  --outputDir  Directory where payload-cert.pem and payload-key.pem will be saved.
			   Defaults to the current working directory.

Examples:
  # Mint an X.509 SVID using current directory as output
  mis mint x509 \
	--spiffeID spiffe://example.org/myservice \

  # Mint an X.509 SVID with DNS SANs, custom TTL, and output directory
  mis mint x509 \
	--spiffeID spiffe://example.org/myservice \
	--dns myservice.example.com \
	--ttl 3600 \
	--outputDir /tmp/svids`,

	RunE: func(cmd *cobra.Command, args []string) error {
		// Default outputDir to current working directory if not provided
		if flags.OutputDir == "" {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to determine current working directory: %w", err)
			}
			flags.OutputDir = cwd
		}

		if err := validateX509Flags(flags); err != nil {
			return err
		}

		fmt.Printf("Minting X.509 SVID with the following parameters:\n")
		fmt.Printf("  SPIFFE ID  : %s\n", flags.SpiffeID)
		fmt.Printf("  CSR Path  : %s\n", flags.CSRPath)
		fmt.Printf("  DNS SANs   : %v\n", flags.DNSNames)
		fmt.Printf(
			"  TTL        : %d seconds (%s)\n",
			flags.TTL,
			time.Duration(flags.TTL)*time.Second,
		)
		fmt.Printf("  Output Dir : %s\n", flags.OutputDir)

		mintx509SVID(&flags)

		return nil
	},
}

func mintx509SVID(flags *x509Flags) {
	cl := client.New(unix.MintUnixSocketPath)
	ctx, cancelFunc := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelFunc()

	ttl := time.Duration(flags.TTL) * time.Second

	cp := filepath.Clean(flags.CSRPath)
	content, err := os.ReadFile(cp)
	if err != nil {
		log.Fatalf("failed to read CSR content, err : %s", err.Error())
	}

	params := &types.MintSVIDRequest{
		DNS:      flags.DNSNames,
		SpiffeID: flags.SpiffeID,
		TTL:      &ttl,
		CSR:      string(content),
	}
	response, err := cl.MintX509SVID(ctx, params)
	if err != nil {
		log.Fatalf("failed to mint x509 SVIDs, err : %s", err.Error())
	}

	// Create files for certificate & key
	// Create folder:
	err = os.MkdirAll(flags.OutputDir, 0o750)
	if err != nil {
		log.Fatalf("failed to create output directory, err: %s", err.Error())
	}

	certContent, err := base64.StdEncoding.DecodeString(response.Certificate)
	if err != nil {
		log.Fatalf(
			"failed to create decode cert file, err: %s",
			err.Error(),
		)
	}
	err = os.WriteFile(path.Join(flags.OutputDir, CertName), certContent, 0o600)
	if err != nil {
		log.Fatalf(
			"failed to create cert file, %s, err: %s",
			path.Join(flags.OutputDir, CertName),
			err.Error(),
		)
	}
	// In case CSR is provided, then key is not returned
	if response.Key != "" {
		certKeyContent, err := base64.StdEncoding.DecodeString(response.Key)
		if err != nil {
			log.Fatalf(
				"failed to create decode cert key file, err: %s",
				err.Error(),
			)
		}

		err = os.WriteFile(path.Join(flags.OutputDir, CertKeyName), certKeyContent, 0o400)
		if err != nil {
			log.Fatalf(
				"failed to create cert key file, %s, err: %s",
				path.Join(flags.OutputDir, CertKeyName),
				err.Error(),
			)
		}
	}
}

func init() {
	// --dns flag: DNS SAN entries to include in the SVID (optional, repeatable)
	x509Cmd.Flags().StringArrayVar(
		&flags.DNSNames,
		"dns",
		[]string{},
		"DNS name to include as a SAN in the SVID (can be specified multiple times)",
	)

	// --spiffeID flag: the SPIFFE ID for the SVID (required when CSR is not provided)
	x509Cmd.Flags().StringVar(
		&flags.SpiffeID,
		"spiffeID",
		"",
		`SPIFFE ID to embed in the SVID. Must follow the format: spiffe://<trust-domain>/<path>
Example: spiffe://example.org/myservice. Required when csrPath is not provided. Else this field is ignored.`,
	)

	// --csrPath flag: path on disk where CSR is present (required when SPIFFE ID is not provided)
	x509Cmd.Flags().StringVar(
		&flags.CSRPath,
		"csrPath",
		"",
		`CSR's Path on disk for minting SVID. It should contain Spiffe Id of principal in URI SAN.`,
	)

	// --ttl flag: validity duration in seconds (optional, defaults to 86400)
	x509Cmd.Flags().IntVar(
		&flags.TTL,
		"ttl",
		defaultTTL,
		"Time-to-live in seconds for the generated SVID (default: 86400 = 24 hours)",
	)

	// --outputDir flag: directory to save the generated SVID and key
	// empty string triggers cwd fallback in RunE
	x509Cmd.Flags().StringVar(
		&flags.OutputDir, "outputDir", "",
		"Directory where payload-cert.pem and payload-key.pem will be saved (default: current working directory). Existing files will be overwritten.",
	)
}

// validateX509Flags performs semantic validation on the parsed x509 flags.
// Cobra handles presence of required flags; this function validates their values.
func validateX509Flags(f x509Flags) error {
	if f.CSRPath != "" {
		cp := filepath.Clean(f.CSRPath)
		content, err := os.ReadFile(cp)
		if err != nil {
			return err
		}
		if err := validateCSR(string(content)); err != nil {
			return err
		}
	}

	// Validate SPIFFE ID format only if CSR is not provided.
	if err := validateSpiffeID(f.SpiffeID); f.CSRPath == "" && err != nil {
		return err
	}

	// Validate TTL is a positive value
	if f.TTL <= 0 {
		return fmt.Errorf("invalid TTL %d: must be a positive integer (seconds)", f.TTL)
	}

	// Validate output directory exists and is accessible
	if err := validateOutputDir(f.OutputDir); err != nil {
		return err
	}

	// Validate each DNS name is non-empty
	for i, dns := range f.DNSNames {
		if strings.TrimSpace(dns) != "" {
			continue
		}
		return fmt.Errorf(
			"DNS name at index %d is empty: DNS names must be non-empty strings",
			i,
		)
	}

	return nil
}

// validateSpiffeID checks that the provided SPIFFE ID conforms to the SPIFFE URI standard.
// A valid SPIFFE ID must:
//   - Use the "spiffe" URI scheme
//   - Have a non-empty trust domain (host)
//   - Have a non-empty path
func validateSpiffeID(id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("spiffeID cannot be empty")
	}

	parsed, err := url.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid spiffeID %q: failed to parse URI: %w", id, err)
	}

	if parsed.Scheme != spiffeScheme {
		return fmt.Errorf(
			"invalid spiffeID %q: scheme must be %q, got %q",
			id, spiffeScheme, parsed.Scheme,
		)
	}

	if parsed.Host == "" {
		return fmt.Errorf(
			"invalid spiffeID %q: trust domain (host) cannot be empty. "+
				"Expected format: spiffe://<trust-domain>/<path>", id,
		)
	}

	if parsed.Path == "" || parsed.Path == "/" {
		return fmt.Errorf(
			"invalid spiffeID %q: path cannot be empty. "+
				"Expected format: spiffe://<trust-domain>/<path>", id,
		)
	}

	return nil
}

// validateOutputDir checks that the specified output directory exists
// and that the current process has write permissions to it.
func validateOutputDir(dir string) error {
	if strings.TrimSpace(dir) == "" {
		return fmt.Errorf("outputDir cannot be empty")
	}

	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return fmt.Errorf("outputDir %q does not exist: please create the directory first", dir)
	}
	if err != nil {
		return fmt.Errorf("outputDir %q is not accessible: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("outputDir %q is not a directory", dir)
	}

	// Check write permission by attempting to create a temp file
	testFile, err := os.CreateTemp(dir, ".mis-write-check-*")
	if err != nil {
		return fmt.Errorf("outputDir %q is not writable: %w", dir, err)
	}
	// Clean up the temp file immediately
	_ = testFile.Close()
	_ = os.Remove(testFile.Name())

	return nil
}

// validateCSR parses and verifies the signature of a PEM or base64-encoded DER CSR.
func validateCSR(csr string) error {
	var derBytes []byte

	block, _ := pem.Decode([]byte(csr))
	if block != nil {
		if block.Type != "CERTIFICATE REQUEST" {
			return fmt.Errorf("PEM block type must be 'CERTIFICATE REQUEST', got '%s'", block.Type)
		}
		derBytes = block.Bytes
	} else {
		var err error
		derBytes, err = base64.StdEncoding.DecodeString(csr)
		if err != nil {
			return fmt.Errorf("must be a valid PEM or base64-encoded DER CSR")
		}
	}

	parsed, err := x509.ParseCertificateRequest(derBytes)
	if err != nil {
		return fmt.Errorf("invalid CSR: %w", err)
	}

	if err := parsed.CheckSignature(); err != nil {
		return fmt.Errorf("CSR signature verification failed: %w", err)
	}

	// Validate URI SANs: exactly one must be present and must be a valid SPIFFE ID
	if len(parsed.URIs) != 1 {
		return fmt.Errorf("CSR must contain exactly one URI SAN, got %d", len(parsed.URIs))
	}

	if err := validateSpiffeID(parsed.URIs[0].String()); err != nil {
		return fmt.Errorf("URI SAN is not a valid SPIFFE ID: %w", err)
	}

	return nil
}
