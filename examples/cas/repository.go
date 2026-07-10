package main

import (
	"context"
	"errors"
	"time"

	"github.com/mkbeh/xredis"
)

var ErrCompareConditionFailed = errors.New("compare condition failed")

type Order struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Version   int64  `json:"version"`
	UpdatedAt string `json:"updated_at"`
}

type StaleSwapResult struct {
	Key          string `json:"key"`
	FirstSwapped bool   `json:"first_swapped"`
	StaleSwapped bool   `json:"stale_swapped"`
	OldOrder     Order  `json:"old_order"`
	FirstOrder   Order  `json:"first_order"`
	StaleOrder   Order  `json:"stale_order"`
	CurrentOrder Order  `json:"current_order"`
}

type orderRepository struct {
	client *xredis.Client
	ttl    time.Duration
}

func newOrderRepository(client *xredis.Client, ttl time.Duration) *orderRepository {
	return &orderRepository{
		client: client,
		ttl:    ttl,
	}
}

func (r *orderRepository) Seed(ctx context.Context, id string) (Order, error) {
	order := Order{
		ID:        id,
		Status:    "processing",
		Version:   1,
		UpdatedAt: nowString(),
	}

	if err := r.client.SetStruct(ctx, orderKey(id), order, r.ttl); err != nil {
		return Order{}, err
	}

	return order, nil
}

func (r *orderRepository) Get(ctx context.Context, id string) (Order, bool, error) {
	var order Order

	ok, err := r.client.GetStruct(ctx, orderKey(id), &order)
	if err != nil {
		return Order{}, false, err
	}

	if !ok {
		return Order{}, false, nil
	}

	return order, true, nil
}

func (r *orderRepository) Complete(ctx context.Context, id string) (Order, bool, error) {
	oldOrder, ok, err := r.Get(ctx, id)
	if err != nil || !ok {
		return Order{}, false, err
	}

	newOrder := nextOrder(oldOrder, "completed")

	swapped, err := r.client.CompareAndSwapStruct(ctx, orderKey(id), oldOrder, newOrder, r.ttl)
	if err != nil {
		return Order{}, false, err
	}

	return newOrder, swapped, nil
}

func (r *orderRepository) DeleteIfCurrent(ctx context.Context, id string) (Order, bool, error) {
	order, ok, err := r.Get(ctx, id)
	if err != nil || !ok {
		return Order{}, false, err
	}

	deleted, err := r.client.CompareAndDeleteStruct(ctx, orderKey(id), order)
	if err != nil {
		return Order{}, false, err
	}

	return order, deleted, nil
}

func (r *orderRepository) StaleSwap(ctx context.Context, id string) (StaleSwapResult, error) {
	oldOrder, err := r.Seed(ctx, id)
	if err != nil {
		return StaleSwapResult{}, err
	}

	firstOrder := nextOrder(oldOrder, "cancelled")
	firstSwapped, err := r.client.CompareAndSwapStruct(ctx, orderKey(id), oldOrder, firstOrder, r.ttl)
	if err != nil {
		return StaleSwapResult{}, err
	}

	staleOrder := nextOrder(oldOrder, "completed")
	staleSwapped, err := r.client.CompareAndSwapStruct(ctx, orderKey(id), oldOrder, staleOrder, r.ttl)
	if err != nil {
		return StaleSwapResult{}, err
	}

	currentOrder, ok, err := r.Get(ctx, id)
	if err != nil {
		return StaleSwapResult{}, err
	}
	if !ok {
		return StaleSwapResult{}, xredis.ErrKeyNotFound
	}

	return StaleSwapResult{
		Key:          orderKey(id),
		FirstSwapped: firstSwapped,
		StaleSwapped: staleSwapped,
		OldOrder:     oldOrder,
		FirstOrder:   firstOrder,
		StaleOrder:   staleOrder,
		CurrentOrder: currentOrder,
	}, nil
}

func nextOrder(order Order, status string) Order {
	order.Status = status
	order.Version++
	order.UpdatedAt = nowString()

	return order
}

func orderKey(id string) string {
	return "xredis:cas:order:" + id
}

func nowString() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}
