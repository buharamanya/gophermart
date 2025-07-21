package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/buharamanya/gophermart/internal/logger"
	"github.com/buharamanya/gophermart/internal/model"
	"github.com/buharamanya/gophermart/internal/service/order"
)

type AccrualWorker struct {
	orderService *order.Service
	client       *http.Client
	address      string
	pollPeriod   time.Duration
}

func NewAccrualWorker(service *order.Service, address string) *AccrualWorker {
	return &AccrualWorker{
		orderService: service,
		client:       &http.Client{Timeout: 5 * time.Second},
		address:      address,
		pollPeriod:   5 * time.Second,
	}
}

func (w *AccrualWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.pollPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Log.Info("accrual worker stopped")
			return
		case <-ticker.C:
			w.processOrders(ctx)
		}
	}
}

func (w *AccrualWorker) processOrders(ctx context.Context) {
	orders, err := w.orderService.GetOrdersForProcessing(ctx)
	if err != nil {
		logger.Log.Error("failed to get orders for processing", zap.Error(err))
		return
	}

	for _, order := range orders {
		select {
		case <-ctx.Done():
			return
		default:
			w.processOrder(ctx, order)
		}
	}
}

func (w *AccrualWorker) processOrder(ctx context.Context, order model.Order) {
	accrualResponse, err := w.getAccrualInfo(ctx, order.Number)
	if err != nil {
		logger.Log.Error("failed to get accrual info",
			zap.String("order", order.Number),
			zap.Error(err))
		return
	}

	switch accrualResponse.Status {
	case "PROCESSED":
		if err := w.orderService.ProcessAccrual(ctx, order.Number, accrualResponse.Status, accrualResponse.Accrual); err != nil {
			logger.Log.Error("failed to process accrual",
				zap.String("order", order.Number),
				zap.Error(err))
		}
	case "INVALID":
		if err := w.orderService.ProcessAccrual(ctx, order.Number, accrualResponse.Status, 0); err != nil {
			logger.Log.Error("failed to process invalid order",
				zap.String("order", order.Number),
				zap.Error(err))
		}
	case "PROCESSING", "REGISTERED":
		// Do nothing, wait for next iteration
	default:
		logger.Log.Warn("unknown order status from accrual system",
			zap.String("order", order.Number),
			zap.String("status", accrualResponse.Status))
	}
}

func (w *AccrualWorker) getAccrualInfo(ctx context.Context, orderNumber string) (*AccrualResponse, error) {
	url := fmt.Sprintf("%s/api/orders/%s", w.address, orderNumber)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var accrualResp AccrualResponse
		if err := json.NewDecoder(resp.Body).Decode(&accrualResp); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		return &accrualResp, nil
	case http.StatusNoContent:
		return nil, errors.New("order not registered in accrual system")
	case http.StatusTooManyRequests:
		retryAfter := resp.Header.Get("Retry-After")
		logger.Log.Warn("rate limit exceeded",
			zap.String("order", orderNumber),
			zap.String("retry_after", retryAfter))
		return nil, errors.New("rate limit exceeded")
	default:
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
}

type AccrualResponse struct {
	Order   string  `json:"order"`
	Status  string  `json:"status"`
	Accrual float64 `json:"accrual,omitempty"`
}
