package withdrawal

import (
	"context"
	"fmt"

	"github.com/buharamanya/gophermart/internal/model"
	"github.com/buharamanya/gophermart/internal/repository"
)

type Service struct {
	repo *repository.Repository
}

func NewService(repo *repository.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetWithdrawals(ctx context.Context, userID int) ([]model.Withdrawal, error) {
	withdrawals, err := s.repo.GetWithdrawalsByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get withdrawals: %w", err)
	}

	return withdrawals, nil
}
