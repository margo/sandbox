package cli

// cli/x509.go
// Defines the "x509" subcommand under "mint" for generating X.509 SVIDs.
// Handles argument parsing, validation, and delegates to the integration layer.

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	// defaultTTL is the default time-to-live for a generated X.509 SVID (24 hours in seconds).
	defaultTTL = 24 * 60 * 60 // 86400 seconds

	// spiffeScheme is the required URI scheme for a valid SPIFFE ID.
	spiffeScheme = "spiffe"
)

// x509Flags holds all parsed flag values for the x509 subcommand.
type x509Flags struct {
	// DNSNames is a list of DNS SANs to include in the X.509 SVID.
	// Can be specified multiple times: --dns foo.example.com --dns bar.example.com
	DNSNames []string

	// SpiffeID is the SPIFFE ID to embed in the SVID.
	// Must follow the format: spiffe://<trust-domain>/<path>
	SpiffeID string

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
  payload-key.pem   — the corresponding private key

Required Flags:
  --spiffeID   The SPIFFE ID to embed in the SVID.
			   Must follow the format: spiffe://<trust-domain>/<path>

Optional Flags:
  --dns        DNS Subject Alternative Name (SAN) to include in the SVID.
			   Can be specified multiple times for multiple DNS names.
  --ttl        Time-to-live in seconds for the SVID. Defaults to 86400 (24 hours).
  --outputDir  Directory where payload-cert.pem and payload-key.pem will be saved.
			   Defaults to the current working directory.

Examples:
  # Mint an X.509 SVID using current directory as output
  svidctl mint x509 \
	--spiffeID spiffe://example.org/myservice \

  # Mint an X.509 SVID with DNS SANs, custom TTL, and output directory
  svidctl mint x509 \
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
		fmt.Printf("  DNS SANs   : %v\n", flags.DNSNames)
		fmt.Printf(
			"  TTL        : %d seconds (%s)\n",
			flags.TTL,
			time.Duration(flags.TTL)*time.Second,
		)
		fmt.Printf("  Output Dir : %s\n", flags.OutputDir)

		// TODO: Integrate X.509 SVID minting logic here

		return nil
	},
}

func init() {
	// --dns flag: DNS SAN entries to include in the SVID (optional, repeatable)
	x509Cmd.Flags().StringArrayVar(
		&flags.DNSNames,
		"dns",
		[]string{},
		"DNS name to include as a SAN in the SVID (can be specified multiple times)",
	)

	// --spiffeID flag: the SPIFFE ID for the SVID (required)
	x509Cmd.Flags().StringVar(
		&flags.SpiffeID,
		"spiffeID",
		"",
		`SPIFFE ID to embed in the SVID. Must follow the format: spiffe://<trust-domain>/<path>
Example: spiffe://example.org/myservice`,
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

	// Mark required flags; cobra will enforce these before RunE is called
	for _, flag := range []string{"spiffeID"} {
		if err := x509Cmd.MarkFlagRequired(flag); err != nil {
			panic(fmt.Sprintf("failed to mark %q flag as required: %v", flag, err))
		}
	}
}

// validateX509Flags performs semantic validation on the parsed x509 flags.
// Cobra handles presence of required flags; this function validates their values.
func validateX509Flags(f x509Flags) error {
	// Validate SPIFFE ID format
	if err := validateSpiffeID(f.SpiffeID); err != nil {
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
	testFile, err := os.CreateTemp(dir, ".svidctl-write-check-*")
	if err != nil {
		return fmt.Errorf("outputDir %q is not writable: %w", dir, err)
	}
	// Clean up the temp file immediately
	_ = testFile.Close()
	_ = os.Remove(testFile.Name())

	return nil
}
