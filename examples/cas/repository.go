package main

import (
	"context"
	"errors"
	"time"

	"github.com/mkbeh/xredis"
)

const orderPrefix = "xredis:cas:order:"

var ErrCompareConditionFailed = errors.New("compare condition failed")

type Order struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Version   int64  `json:"version"`
	UpdatedAt string `json:"updated_at"`
}

type StaleSwapResult struct {
	Key              string          `json:"key"`
	OriginalOrder    Order           `json:"original_order"`
	OriginalRevision xredis.Revision `json:"original_revision"`
	FirstOrder       Order           `json:"first_order"`
	FirstRevision    xredis.Revision `json:"first_revision"`
	FirstSwapped     bool            `json:"first_swapped"`
	StaleOrder       Order           `json:"stale_order"`
	StaleRevision    xredis.Revision `json:"stale_revision"`
	StaleSwapped     bool            `json:"stale_swapped"`
	CurrentOrder     Order           `json:"current_order"`
	CurrentRevision  xredis.Revision `json:"current_revision"`
}

type orderRepository struct {
	client *xredis.Client
	store  *xredis.VersionedStore[Order]
	ttl    time.Duration
}

func newOrderRepository(
	client *xredis.Client,
	ttl time.Duration,
) (*orderRepository, error) {
	store, err := xredis.NewVersionedStore[Order](
		client,
		xredis.WithVersionedStorePrefix(orderPrefix),
	)
	if err != nil {
		return nil, err
	}

	return &orderRepository{
		client: client,
		store:  store,
		ttl:    ttl,
	}, nil
}

func (r *orderRepository) Seed(
	ctx context.Context,
	id string,
) (xredis.VersionedValue[Order], bool, error) {
	order := Order{
		ID:        id,
		Status:    "processing",
		Version:   1,
		UpdatedAt: nowString(),
	}

	revision, created, err := r.store.SetIfAbsent(
		ctx,
		id,
		order,
		r.ttl,
	)
	if err != nil {
		return xredis.VersionedValue[Order]{}, false, err
	}

	if !created {
		return xredis.VersionedValue[Order]{}, false, nil
	}

	return xredis.VersionedValue[Order]{
		Value:    order,
		Revision: revision,
	}, true, nil
}

func (r *orderRepository) Get(
	ctx context.Context,
	id string,
) (xredis.VersionedValue[Order], bool, error) {
	return r.store.Get(ctx, id)
}

func (r *orderRepository) Complete(
	ctx context.Context,
	id string,
) (xredis.VersionedValue[Order], bool, error) {
	entry, ok, err := r.Get(ctx, id)
	if err != nil {
		return xredis.VersionedValue[Order]{}, false, err
	}
	if !ok {
		return xredis.VersionedValue[Order]{}, false, xredis.ErrKeyNotFound
	}

	updated := nextOrder(entry.Value, "completed")

	revision, swapped, err := r.store.CompareAndSwap(
		ctx,
		id,
		entry.Revision,
		updated,
		xredis.KeepTTL,
	)
	if err != nil {
		return xredis.VersionedValue[Order]{}, false, err
	}

	if !swapped {
		return xredis.VersionedValue[Order]{}, false, nil
	}

	return xredis.VersionedValue[Order]{
		Value:    updated,
		Revision: revision,
	}, true, nil
}

func (r *orderRepository) DeleteIfCurrent(
	ctx context.Context,
	id string,
) (xredis.VersionedValue[Order], bool, error) {
	entry, ok, err := r.Get(ctx, id)
	if err != nil {
		return xredis.VersionedValue[Order]{}, false, err
	}
	if !ok {
		return xredis.VersionedValue[Order]{}, false, xredis.ErrKeyNotFound
	}

	deleted, err := r.store.CompareAndDelete(
		ctx,
		id,
		entry.Revision,
	)
	if err != nil {
		return xredis.VersionedValue[Order]{}, false, err
	}

	return entry, deleted, nil
}

func (r *orderRepository) StaleSwap(
	ctx context.Context,
	id string,
) (StaleSwapResult, error) {
	if err := r.client.Delete(ctx, orderKey(id)); err != nil {
		return StaleSwapResult{}, err
	}

	original, created, err := r.Seed(ctx, id)
	if err != nil {
		return StaleSwapResult{}, err
	}
	if !created {
		return StaleSwapResult{}, ErrCompareConditionFailed
	}

	firstOrder := nextOrder(original.Value, "cancelled")

	firstRevision, firstSwapped, err := r.store.CompareAndSwap(
		ctx,
		id,
		original.Revision,
		firstOrder,
		xredis.KeepTTL,
	)
	if err != nil {
		return StaleSwapResult{}, err
	}
	if !firstSwapped {
		return StaleSwapResult{}, ErrCompareConditionFailed
	}

	staleOrder := nextOrder(original.Value, "completed")

	staleRevision, staleSwapped, err := r.store.CompareAndSwap(
		ctx,
		id,
		original.Revision,
		staleOrder,
		xredis.KeepTTL,
	)
	if err != nil {
		return StaleSwapResult{}, err
	}

	current, ok, err := r.Get(ctx, id)
	if err != nil {
		return StaleSwapResult{}, err
	}
	if !ok {
		return StaleSwapResult{}, xredis.ErrKeyNotFound
	}

	return StaleSwapResult{
		Key:              orderKey(id),
		OriginalOrder:    original.Value,
		OriginalRevision: original.Revision,
		FirstOrder:       firstOrder,
		FirstRevision:    firstRevision,
		FirstSwapped:     firstSwapped,
		StaleOrder:       staleOrder,
		StaleRevision:    staleRevision,
		StaleSwapped:     staleSwapped,
		CurrentOrder:     current.Value,
		CurrentRevision:  current.Revision,
	}, nil
}

func nextOrder(order Order, status string) Order {
	order.Status = status
	order.Version++
	order.UpdatedAt = nowString()

	return order
}

func orderKey(id string) string {
	return orderPrefix + id
}

func nowString() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}
