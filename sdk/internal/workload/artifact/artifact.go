package artifact

// ArtifactHandler is the interface for operations on artifacts (pull, push, verify, etc.)
type ArtifactHandler interface {
	Pull(source string, dest string, opts ...Option) error
	Push(source string, dest string, opts ...Option) error
	Verify(source string, opts ...Option) error
}

type Option interface{}
