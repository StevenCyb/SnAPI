package api

import (
	"fmt"
	"sync"

	"github.com/StevenCyb/example_proto/pkg/api/model"
	"github.com/google/uuid"
)

// todoStore is a minimal thread-safe in-memory store -- enough business logic
// to make the generated handlers actually do something. Not part of what
// `snapi proto` generates; hand-written like the rest of the handler bodies.
type todoStore struct {
	mu    sync.RWMutex
	items map[string]model.Todo
}

func (s *todoStore) list() []model.Todo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.Todo, 0, len(s.items))
	for _, t := range s.items {
		out = append(out, t)
	}
	return out
}

func (s *todoStore) get(id string) (model.Todo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.items[id]
	return t, ok
}

func (s *todoStore) create(title string) model.Todo {
	s.mu.Lock()
	defer s.mu.Unlock()
	t := model.Todo{Id: uuid.NewString(), Title: title}
	s.items[t.Id] = t
	return t
}

func (s *todoStore) delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.items[id]
	delete(s.items, id)
	return ok
}

var todos *todoStore

// @snapi.setup
func Setup() {
	fmt.Println("Setting up in-memory todo store...")
	todos = &todoStore{items: make(map[string]model.Todo)}
}

// @snapi.teardown
func Teardown() {
	fmt.Println("Tearing down...")
	todos = nil
}
