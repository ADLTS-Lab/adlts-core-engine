package runtime

import (
	"log/slog"

	"adlts/internal/platform/config"
	"adlts/internal/platform/security"
	"adlts/internal/platform/store"
)

type Dependencies struct {
	Config config.Config
	Store  *store.Store
	Tokens *security.Manager
	Logger *slog.Logger
}
