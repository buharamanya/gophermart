package balance

import (
	"context"
	"fmt"

	"github.com/buharamanya/gophermart/internal/model"
	"github.com/buharamanya/gophermart/internal/repository"
	"github.com/buharamanya/gophermart/internal/util"
)

type Service struct {
	repo *repository.Repository
}

func NewService(repo *repository.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetBalance(ctx context.Context, userID int) (*model.Balance, error) {
	balance, err := s.repo.GetBalance(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}

	return balance, nil
}

func (s *Service) Withdraw(ctx context.Context, userID int, orderNumber string, sum float64) error {

	if !util.ValidateLuhn(orderNumber) {
		return util.ErrInvalidOrderNumber
	}

	withdrawal := &model.Withdrawal{
		UserID:      userID,
		OrderNumber: orderNumber,
		Sum:         sum,
	}

	if err := s.repo.CreateWithdrawal(ctx, withdrawal); err != nil {
		return fmt.Errorf("failed to create withdrawal: %w", err)
	}

	return nil
}
