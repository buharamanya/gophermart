package order

import (
	"context"
	"errors"
	"fmt"

	"github.com/buharamanya/gophermart/internal/model"
	"github.com/buharamanya/gophermart/internal/repository"
	"github.com/buharamanya/gophermart/internal/util"
)

var (
	ErrInvalidOrderNumber = errors.New("invalid order number")
	ErrOrderConflict      = errors.New("order conflict")
)

type Service struct {
	repo *repository.Repository
}

func NewService(repo *repository.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) UploadOrder(ctx context.Context, userID int, number string) error {
	if !util.ValidateLuhn(number) {
		return ErrInvalidOrderNumber
	}

	order := &model.Order{
		UserID: userID,
		Number: number,
		Status: "NEW",
	}

	if err := s.repo.CreateOrder(ctx, order); err != nil {
		if errors.Is(err, repository.ErrOrderExists) {
			return err
		}
		if errors.Is(err, repository.ErrOrderNotOwned) {
			return err
		}
		return fmt.Errorf("create order: %w", err)
	}

	return nil
}

func (s *Service) GetOrders(ctx context.Context, userID int) ([]model.Order, error) {
	orders, err := s.repo.GetOrdersByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get orders: %w", err)
	}
	return orders, nil
}
