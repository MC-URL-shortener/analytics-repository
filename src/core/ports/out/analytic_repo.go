package out

import (
	"context"

	"gitlab.com/URL-shortener4224128/analytics-repository/src/core/dto"
	"gitlab.com/URL-shortener4224128/analytics-repository/src/core/models"
)

type AnalyticRepo interface {
	Save(ctx context.Context, obj *models.Analytic) error
	GetAnalyticsByDevice(ctx context.Context, query *dto.Query) (*dto.Paginate[*dto.Device], error)
}
