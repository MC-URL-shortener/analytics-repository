package usecases

import (
	"context"

	"gitlab.com/URL-shortener4224128/analytics-repository/src/core/models"
	"gitlab.com/URL-shortener4224128/analytics-repository/src/core/ports/out"
)

type SaveAnalyticsUseCase struct {
	repo out.AnalyticRepo
}

func NewSaveAnalyticsUseCase(
	repo out.AnalyticRepo,
) *SaveAnalyticsUseCase {
	return &SaveAnalyticsUseCase{
		repo: repo,
	}
}

func (s *SaveAnalyticsUseCase) Execute(ctx context.Context, obj *models.Analytic) error {
	err := s.repo.Save(
		ctx,
		obj,
	)

	if err != nil {
		return err
	}

	return nil
}
