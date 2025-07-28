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
	ErrOrderConflict = errors.New("order conflict")
)

type Service struct {
	repo *repository.Repository
}

func NewService(repo *repository.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) UploadOrder(ctx context.Context, userID int, number string) error {
	if !util.ValidateLuhn(number) {
		return util.ErrInvalidOrderNumber
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

func (s *Service) GetOrdersForProcessing(ctx context.Context) ([]model.Order, error) {
	// Получаем заказы со статусом NEW или PROCESSING
	orders, err := s.repo.GetOrdersByStatus(ctx, []string{"NEW", "PROCESSING"})
	if err != nil {
		return nil, fmt.Errorf("failed to get processing orders: %w", err)
	}
	return orders, nil
}

func (s *Service) ProcessAccrual(ctx context.Context, orderNumber string, status string, accrual float64) error {
	// Начинаем транзакцию
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback(ctx)
		}
	}()

	// 1. Обновляем статус заказа
	if err = s.repo.UpdateOrderStatus(ctx, orderNumber, status, accrual); err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}

	// Если заказ обработан и есть начисление - обновляем баланс
	if status == "PROCESSED" && accrual > 0 {
		// 2. Получаем заказ для получения userID
		order, err := s.repo.GetOrderByNumber(ctx, orderNumber)
		if err != nil {
			return fmt.Errorf("failed to get order: %w", err)
		}

		// 3. Получаем текущий баланс
		balance, err := s.repo.GetBalance(ctx, order.UserID)
		if err != nil {
			return fmt.Errorf("failed to get balance: %w", err)
		}

		// 4. Обновляем баланс
		newCurrent := balance.Current + accrual
		if err = s.repo.UpdateBalance(ctx, order.UserID, newCurrent, balance.Withdrawn); err != nil {
			return fmt.Errorf("failed to update balance: %w", err)
		}
	}

	// Коммитим транзакцию
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
