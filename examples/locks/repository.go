package main

import (
	"context"
	"errors"
	"sync"
	"time"
)

const repositoryDelay = 300 * time.Millisecond

var ErrStaleFencingToken = errors.New("stale fencing token")

type Order struct {
	ID               string `json:"id"`
	Status           string `json:"status"`
	Version          int64  `json:"version"`
	LastFencingToken int64  `json:"last_fencing_token"`
}

type orderRepository struct {
	mu     sync.Mutex
	orders map[string]Order
}

func newOrderRepository() *orderRepository {
	return &orderRepository{
		orders: seedOrders(),
	}
}

func (r *orderRepository) Get(id string) (Order, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	order, ok := r.orders[id]

	return order, ok
}

func (r *orderRepository) Process(ctx context.Context, id, status string) (Order, error) {
	if err := sleepContext(ctx, repositoryDelay); err != nil {
		return Order{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	order := r.order(id)
	order.Status = status
	order.Version++

	r.orders[id] = order

	return order, nil
}

func (r *orderRepository) ProcessWithFence(ctx context.Context, id string, fencingToken int64, status string) (Order, error) {
	if err := sleepContext(ctx, repositoryDelay); err != nil {
		return Order{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	order := r.order(id)
	if fencingToken <= order.LastFencingToken {
		return Order{}, ErrStaleFencingToken
	}

	order.Status = status
	order.Version++
	order.LastFencingToken = fencingToken

	r.orders[id] = order

	return order, nil
}

func (r *orderRepository) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.orders = seedOrders()
}

func (r *orderRepository) order(id string) Order {
	order, ok := r.orders[id]
	if ok {
		return order
	}

	return Order{
		ID:     id,
		Status: "new",
	}
}

func seedOrders() map[string]Order {
	return map[string]Order{
		"42": {
			ID:     "42",
			Status: "new",
		},
		"7": {
			ID:     "7",
			Status: "new",
		},
		"100": {
			ID:     "100",
			Status: "new",
		},
	}
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
