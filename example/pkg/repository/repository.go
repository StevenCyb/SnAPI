package repository

// Item is an interface that must provide an ID.
type Item interface {
	GetID() string
}

// Repository provides basic CRUD operations for Items.
type Repository[T Item] interface {
	Get(id string) (T, bool)
	List() []T
	Update(item T) bool
	Delete(id string) bool
}

type memoryRepository[T Item] struct {
	items map[string]T
}

func NewMemoryRepository[T Item]() Repository[T] {
	return &memoryRepository[T]{items: make(map[string]T)}
}

func (r *memoryRepository[T]) Get(id string) (T, bool) {
	item, ok := r.items[id]
	return item, ok
}

func (r *memoryRepository[T]) List() []T {
	result := make([]T, 0, len(r.items))
	for _, item := range r.items {
		result = append(result, item)
	}
	return result
}

func (r *memoryRepository[T]) Update(item T) bool {
	id := item.GetID()
	_, exists := r.items[id]
	r.items[id] = item
	return exists
}

func (r *memoryRepository[T]) Delete(id string) bool {
	_, exists := r.items[id]
	if exists {
		delete(r.items, id)
	}
	return exists
}
