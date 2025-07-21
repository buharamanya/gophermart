package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/buharamanya/gophermart/internal/model"
)

var (
	ErrUserExists          = errors.New("user already exists")
	ErrUserNotFound        = errors.New("user not found")
	ErrOrderExists         = errors.New("order already exists")
	ErrOrderNotOwned       = errors.New("order belongs to another user")
	ErrOrderNotFound       = errors.New("order not found")
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrWithdrawalExists    = errors.New("withdrawal already exists")
)

type Repository struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// UserRepository methods

func (r *Repository) CreateUser(ctx context.Context, user *model.User) error {
	query := `
		INSERT INTO users (login, password_hash)
		VALUES ($1, $2)
		RETURNING id, created_at`

	err := r.db.QueryRow(ctx, query, user.Login, user.PasswordHash).
		Scan(&user.ID, &user.CreatedAt)

	if err != nil {
		if isDuplicateKeyError(err) {
			return ErrUserExists
		}
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

func (r *Repository) GetUserByLogin(ctx context.Context, login string) (*model.User, error) {
	query := `
		SELECT id, login, password_hash, created_at
		FROM users
		WHERE login = $1`

	var user model.User
	err := r.db.QueryRow(ctx, query, login).
		Scan(&user.ID, &user.Login, &user.PasswordHash, &user.CreatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// OrderRepository methods

func (r *Repository) CreateOrder(ctx context.Context, order *model.Order) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var existingUserID int
	err = tx.QueryRow(ctx,
		"SELECT user_id FROM orders WHERE number = $1",
		order.Number).Scan(&existingUserID)

	switch {
	case err == nil:
		if existingUserID == order.UserID {
			return ErrOrderExists
		}
		return ErrOrderNotOwned
	case errors.Is(err, pgx.ErrNoRows):
		// Order doesn't exist, proceed with creation
	default:
		return fmt.Errorf("failed to check order existence: %w", err)
	}

	err = tx.QueryRow(ctx,
		`INSERT INTO orders (user_id, number, status)
		 VALUES ($1, $2, $3)
		 RETURNING id, uploaded_at`,
		order.UserID, order.Number, order.Status).
		Scan(&order.ID, &order.UploadedAt)

	if err != nil {
		return fmt.Errorf("failed to create order: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (r *Repository) GetOrdersByUser(ctx context.Context, userID int) ([]model.Order, error) {
	query := `
		SELECT id, user_id, number, status, accrual, uploaded_at, processed_at
		FROM orders
		WHERE user_id = $1
		ORDER BY uploaded_at DESC`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query orders: %w", err)
	}
	defer rows.Close()

	var orders []model.Order
	for rows.Next() {
		var o model.Order
		if err := rows.Scan(
			&o.ID, &o.UserID, &o.Number, &o.Status,
			&o.Accrual, &o.UploadedAt, &o.ProcessedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan order: %w", err)
		}
		orders = append(orders, o)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return orders, nil
}

func (r *Repository) UpdateOrderStatus(ctx context.Context, number string, status string, accrual float64) error {
	query := `
		UPDATE orders
		SET status = $1, accrual = $2, processed_at = CURRENT_TIMESTAMP
		WHERE number = $3`

	_, err := r.db.Exec(ctx, query, status, accrual, number)
	if err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}

	return nil
}

// BalanceRepository methods

func (r *Repository) GetBalance(ctx context.Context, userID int) (*model.Balance, error) {
	query := `
		SELECT current, withdrawn, updated_at
		FROM balances
		WHERE user_id = $1`

	var balance model.Balance
	balance.UserID = userID

	err := r.db.QueryRow(ctx, query, userID).
		Scan(&balance.Current, &balance.Withdrawn, &balance.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Return zero balance if not found
			return &model.Balance{
				UserID:    userID,
				Current:   0,
				Withdrawn: 0,
				UpdatedAt: time.Now(),
			}, nil
		}
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}

	return &balance, nil
}

func (r *Repository) UpdateBalance(ctx context.Context, userID int, current, withdrawn float64) error {
	query := `
		INSERT INTO balances (user_id, current, withdrawn)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id) DO UPDATE
		SET current = EXCLUDED.current,
		    withdrawn = EXCLUDED.withdrawn,
		    updated_at = CURRENT_TIMESTAMP`

	_, err := r.db.Exec(ctx, query, userID, current, withdrawn)
	if err != nil {
		return fmt.Errorf("failed to update balance: %w", err)
	}

	return nil
}

// WithdrawalRepository methods

func (r *Repository) CreateWithdrawal(ctx context.Context, withdrawal *model.Withdrawal) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Check if withdrawal already exists
	var exists bool
	err = tx.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM withdrawals WHERE order_number = $1)",
		withdrawal.OrderNumber).Scan(&exists)

	if err != nil {
		return fmt.Errorf("failed to check withdrawal existence: %w", err)
	}

	if exists {
		return ErrWithdrawalExists
	}

	// Check balance
	var currentBalance float64
	err = tx.QueryRow(ctx,
		"SELECT current FROM balances WHERE user_id = $1",
		withdrawal.UserID).Scan(&currentBalance)

	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("failed to get balance: %w", err)
	}

	if currentBalance < withdrawal.Sum {
		return ErrInsufficientBalance
	}

	// Create withdrawal
	err = tx.QueryRow(ctx,
		`INSERT INTO withdrawals (user_id, order_number, sum)
		 VALUES ($1, $2, $3)
		 RETURNING id, processed_at`,
		withdrawal.UserID, withdrawal.OrderNumber, withdrawal.Sum).
		Scan(&withdrawal.ID, &withdrawal.ProcessedAt)

	if err != nil {
		return fmt.Errorf("failed to create withdrawal: %w", err)
	}

	// Update balance
	_, err = tx.Exec(ctx,
		`INSERT INTO balances (user_id, current, withdrawn)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (user_id) DO UPDATE
		 SET current = balances.current - EXCLUDED.current,
		     withdrawn = balances.withdrawn + EXCLUDED.withdrawn,
		     updated_at = CURRENT_TIMESTAMP`,
		withdrawal.UserID, withdrawal.Sum, withdrawal.Sum)

	if err != nil {
		return fmt.Errorf("failed to update balance: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (r *Repository) GetWithdrawalsByUser(ctx context.Context, userID int) ([]model.Withdrawal, error) {
	query := `
		SELECT id, user_id, order_number, sum, processed_at
		FROM withdrawals
		WHERE user_id = $1
		ORDER BY processed_at DESC`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query withdrawals: %w", err)
	}
	defer rows.Close()

	var withdrawals []model.Withdrawal
	for rows.Next() {
		var w model.Withdrawal
		if err := rows.Scan(
			&w.ID, &w.UserID, &w.OrderNumber, &w.Sum, &w.ProcessedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan withdrawal: %w", err)
		}
		withdrawals = append(withdrawals, w)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return withdrawals, nil
}

// Helper function
func isDuplicateKeyError(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// Добавляем в Repository
func (r *Repository) GetOrdersByStatus(ctx context.Context, statuses []string) ([]model.Order, error) {
	query := `
		SELECT id, user_id, number, status, accrual, uploaded_at, processed_at
		FROM orders
		WHERE status = ANY($1)
		ORDER BY uploaded_at ASC`

	rows, err := r.db.Query(ctx, query, statuses)
	if err != nil {
		return nil, fmt.Errorf("failed to query orders: %w", err)
	}
	defer rows.Close()

	var orders []model.Order
	for rows.Next() {
		var o model.Order
		if err := rows.Scan(
			&o.ID, &o.UserID, &o.Number, &o.Status,
			&o.Accrual, &o.UploadedAt, &o.ProcessedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan order: %w", err)
		}
		orders = append(orders, o)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return orders, nil
}

// GetOrderByNumber возвращает заказ по номеру
func (r *Repository) GetOrderByNumber(ctx context.Context, number string) (*model.Order, error) {
	query := `
        SELECT id, user_id, number, status, accrual, uploaded_at, processed_at
        FROM orders
        WHERE number = $1`

	var order model.Order
	err := r.db.QueryRow(ctx, query, number).
		Scan(&order.ID, &order.UserID, &order.Number, &order.Status,
			&order.Accrual, &order.UploadedAt, &order.ProcessedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrOrderNotFound
		}
		return nil, fmt.Errorf("failed to get order: %w", err)
	}

	return &order, nil
}

// BeginTx начинает новую транзакцию
func (r *Repository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	return tx, nil
}
