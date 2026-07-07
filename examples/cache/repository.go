package main

import (
	"context"
	"sync"
	"time"

	"github.com/mkbeh/xredis"
)

const repositoryDelay = 300 * time.Millisecond

type userRepository struct {
	mu    sync.Mutex
	data  map[string]User
	loads map[string]int
}

func newUserRepository() *userRepository {
	return &userRepository{
		data:  seedUsers(),
		loads: make(map[string]int),
	}
}

func (r *userRepository) GetByID(ctx context.Context, id string) (User, error) {
	timer := time.NewTimer(repositoryDelay)
	defer timer.Stop()

	select {
	case <-timer.C:
	case <-ctx.Done():
		return User{}, ctx.Err()
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.loads[id]++

	user, ok := r.data[id]
	if !ok {
		return User{}, xredis.ErrKeyNotFound
	}

	return user, nil
}

func (r *userRepository) Set(user User) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.data[user.ID] = user
}

func (r *userRepository) Delete(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.data, id)
}

func (r *userRepository) Loads() map[string]int {
	r.mu.Lock()
	defer r.mu.Unlock()

	loads := make(map[string]int, len(r.loads))
	for key, value := range r.loads {
		loads[key] = value
	}

	return loads
}

func (r *userRepository) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.data = seedUsers()
	r.loads = make(map[string]int)
}

func seedUsers() map[string]User {
	return map[string]User{
		"42": {
			ID:     "42",
			Name:   "Ada Lovelace from repository",
			Age:    36,
			Active: true,
		},
		"7": {
			ID:     "7",
			Name:   "Grace Hopper from repository",
			Age:    85,
			Active: true,
		},
	}
}
