package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
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
	workerCount  int
	rateLimit    *RateLimiter
}

type RateLimiter struct {
	mu            sync.Mutex
	retryAfter    time.Duration
	limited       bool
	lastLimitTime time.Time
}

func NewAccrualWorker(service *order.Service, address string, workerCount int) *AccrualWorker {
	return &AccrualWorker{
		orderService: service,
		client:       &http.Client{Timeout: 5 * time.Second},
		address:      address,
		pollPeriod:   5 * time.Second,
		workerCount:  workerCount,
		rateLimit:    &RateLimiter{},
	}
}

func (w *AccrualWorker) Start(ctx context.Context) {
	ordersCh := make(chan model.Order, w.workerCount*2)
	var wg sync.WaitGroup

	// Запуск воркеров
	for i := 0; i < w.workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.worker(ctx, ordersCh)
		}()
	}

	// Главный цикл обработки
	ticker := time.NewTicker(w.pollPeriod)
	defer func() {
		ticker.Stop()
		close(ordersCh)
		wg.Wait()
		logger.Log.Info("all accrual workers stopped")
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if w.rateLimit.IsLimited() {
				continue
			}

			orders, err := w.orderService.GetOrdersForProcessing(ctx)
			if err != nil {
				logger.Log.Error("failed to get orders for processing", zap.Error(err))
				continue
			}

			for _, order := range orders {
				select {
				case ordersCh <- order:
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

func (w *AccrualWorker) worker(ctx context.Context, ordersCh <-chan model.Order) {
	for order := range ordersCh {
		select {
		case <-ctx.Done():
			return
		default:
			if w.rateLimit.IsLimited() {
				continue
			}

			w.processOrder(ctx, order)
		}
	}
}

func (w *AccrualWorker) processOrder(ctx context.Context, order model.Order) {
	accrualResponse, err := w.getAccrualInfo(ctx, order.Number)
	if err != nil {
		if errors.Is(err, ErrRateLimitExceeded) {
			w.rateLimit.SetLimited(true)
		}
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
		// Ничего не делаем, ждем следующей итерации
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
		retryAfter := w.parseRetryAfter(resp.Header.Get("Retry-After"))
		logger.Log.Warn("rate limit exceeded",
			zap.String("order", orderNumber),
			zap.Duration("retry_after", retryAfter))
		return nil, ErrRateLimitExceeded
	default:
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
}

func (w *AccrualWorker) parseRetryAfter(header string) time.Duration {
	if header == "" {
		return 60 * time.Second // default value
	}

	seconds, err := strconv.Atoi(header)
	if err != nil {
		return 60 * time.Second
	}

	return time.Duration(seconds) * time.Second
}

func (rl *RateLimiter) IsLimited() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if !rl.limited {
		return false
	}

	if time.Since(rl.lastLimitTime) >= rl.retryAfter {
		rl.limited = false
		return false
	}

	return true
}

func (rl *RateLimiter) SetLimited(limited bool) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.limited = limited
	if limited {
		rl.lastLimitTime = time.Now()
	}
}

var ErrRateLimitExceeded = errors.New("rate limit exceeded")

type AccrualResponse struct {
	Order   string  `json:"order"`
	Status  string  `json:"status"`
	Accrual float64 `json:"accrual,omitempty"`
}
