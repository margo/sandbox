package helm

type BasicAuthCredentials struct {
	Username string
	Password string
}

type TokenCredentials struct {
	Token string
	Type  string // e.g., "Bearer"
}

type ChartPullCredentialsType string

// ChartPullCredentials represents the credentials needed to pull a Helm chart from a repository.
// It can be either basic authentication or token-based authentication.
// Supported types are "basic" for BasicAuthCredentials and "token" for TokenCredentials.
// ChartPullCredentialsType is an enum for the type of credentials used for pulling Helm charts.
// It can be either "basic" for BasicAuthCredentials or "token" for TokenCredentials.
const (
	ChartPullCredentialsTypeBasic = "basic"
	ChartPullCredentialsTypeToken = "token"
)

type ChartPullCredentials struct {
	// add one of the following
	Type                 ChartPullCredentialsType
	BasicAuthCredentials *BasicAuthCredentials
	TokenCredentials     *TokenCredentials
}

func PullChart(chartURI string, creds ChartPullCredentials) error {
	return nil
}
