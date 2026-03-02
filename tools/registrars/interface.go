package registrars

import (
	"context"

	"github.com/fenilmodi00/ipo-backend/shared"
)

type RegistrarClient interface {
	GetActiveIPOs(ctx context.Context) ([]shared.DropdownOption, error)
	MatchCompanyName(ipoName string, options []shared.DropdownOption) (string, float64)
	CheckAllotment(ctx context.Context, companyCode, pan string) (*shared.AllotmentResult, error)
}
