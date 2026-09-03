package operations

import "log/slog"

type MintOperations struct {
	logger *slog.Logger
}

func New(logger *slog.Logger) *MintOperations {
	return &MintOperations{
		logger: logger,
	}
}
