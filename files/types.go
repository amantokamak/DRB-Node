package files

import (
	"context"

	"github.com/tokamak-network/DRB-Node/utils"
)

// PoFClientWrapper wraps the PoFClient.
type PoFClientWrapper struct {
	*utils.PoFClient
}

// NewPoFClientWrapper creates a new PoFClientWrapper.
func NewPoFClientWrapper(pofClient *utils.PoFClient) *PoFClientWrapper {
	return &PoFClientWrapper{PoFClient: pofClient}
}

// ProcessRoundResults delegates to the PoFClient method.
func (l *PoFClientWrapper) ProcessRoundResults(ctx context.Context) error {
	return l.PoFClient.ProcessRoundResults(ctx)
}
