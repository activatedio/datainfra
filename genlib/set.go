package genlib

import "sync"

// Set is a generic thread-safe collection type for managing unique elements of comparable types.
type Set[T comparable] struct {
	mu      sync.Mutex
	entries map[T]struct{}
	frozen  bool
}

// ensureEntries initializes the entries map if it is nil. It is a helper to ensure the internal structure is ready for use.
func (s *Set[T]) ensureEntries() {
	if s.entries == nil {
		s.entries = make(map[T]struct{})
	}
}

// Add inserts one or more values into the set. If the set is frozen, it will panic. Duplicate values are ignored.
func (s *Set[T]) Add(values ...T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureEntries()
	if s.frozen {
		panic("set is frozen")
	}
	for _, v := range values {
		s.entries[v] = struct{}{}
	}
}

// Remove deletes the specified values from the set. Panics if the set is frozen.
func (s *Set[T]) Remove(values ...T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureEntries()
	if s.frozen {
		panic("set is frozen")
	}
	for _, v := range values {
		delete(s.entries, v)
	}
}

// Contains checks if the specified value exists in the set and returns true if found, otherwise false.
func (s *Set[T]) Contains(value T) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureEntries()
	_, ok := s.entries[value]
	return ok
}

// Freeze makes the set immutable, preventing any further modifications to its entries. It uses a mutex to ensure thread safety.
func (s *Set[T]) Freeze() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.frozen = true
}

// Clone creates and returns a new Set with a copy of the entries from the original Set. The new Set is not frozen.
func (s *Set[T]) Clone() *Set[T] {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureEntries()
	tmp := map[T]struct{}{}

	for k, v := range s.entries {
		tmp[k] = v
	}
	return &Set[T]{
		mu:      sync.Mutex{},
		entries: map[T]struct{}{},
		frozen:  false,
	}
}

// Intersect creates a new Set containing elements present in both the receiver Set and the provided Set.
func (s *Set[T]) Intersect(other *Set[T]) *Set[T] {

	res := NewSet[T]()

	for _, el := range other.All() {
		if s.Contains(el) {
			res.Add(el)
		}
	}

	return res
}

// Len returns the number of elements currently stored in the set. It is safe for concurrent use.
func (s *Set[T]) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entries)
}

// All returns a slice containing all elements currently present in the set. The order of elements is not guaranteed.
func (s *Set[T]) All() []T {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureEntries()
	res := make([]T, len(s.entries))
	i := 0
	for k := range s.entries {
		res[i] = k
		i++
	}
	return res
}

// NewSet creates and returns a new instance of a Set initialized with the provided entries.
func NewSet[T comparable](entries ...T) *Set[T] {

	s := &Set[T]{}
	s.Add(entries...)
	return s
}

// NewFrozenSet creates a new frozen set initialized with the provided entries. The set becomes immutable after creation.
func NewFrozenSet[T comparable](entries ...T) *Set[T] {

	s := NewSet[T](entries...)
	s.Freeze()
	return s
}
