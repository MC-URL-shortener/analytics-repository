package usecases

import (
	"context"

	"gitlab.com/URL-shortener4224128/analytics-repository/src/core/dto"
	"gitlab.com/URL-shortener4224128/analytics-repository/src/core/ports/out"
)

type GetAnalyticsByDeviceUseCase struct {
	repo out.AnalyticRepo
}

func NewGetAnalyticsByDeviceUseCase(repo out.AnalyticRepo) *GetAnalyticsByDeviceUseCase {
	return &GetAnalyticsByDeviceUseCase{
		repo: repo,
	}
}

func (g *GetAnalyticsByDeviceUseCase) Execute(ctx context.Context, query *dto.Query) (*dto.Paginate[*dto.Device], error) {
	devices, err := g.repo.GetAnalyticsByDevice(ctx, query)

	if err != nil {
		return nil, err
	}

	return devices, nil
}
