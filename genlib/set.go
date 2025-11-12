package genlib

import "sync"

type Set[T comparable] struct {
	mu      sync.Mutex
	entries map[T]struct{}
	frozen  bool
}

// ensureEntries ensures entries is setup
// must be guarded by s.mu
func (s *Set[T]) ensureEntries() {
	if s.entries == nil {
		s.entries = make(map[T]struct{})
	}
}

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

func (s *Set[T]) Contains(value T) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureEntries()
	_, ok := s.entries[value]
	return ok
}

// Freeze prevents more entries from being added
func (s *Set[T]) Freeze() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.frozen = true
}

// Clone copies entries from this set into a new set, returning an unfrozen set
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

func (s *Set[T]) Intersect(other *Set[T]) *Set[T] {

	res := NewSet[T]()

	for _, el := range other.All() {
		if s.Contains(el) {
			res.Add(el)
		}
	}

	return res
}

func (s *Set[T]) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entries)
}

func (s *Set[T]) All() []T {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureEntries()
	var res []T
	for k, _ := range s.entries {
		res = append(res, k)
	}
	return res
}

func NewSet[T comparable](entries ...T) *Set[T] {

	s := &Set[T]{}
	s.Add(entries...)
	return s
}
func NewFrozenSet[T comparable](entries ...T) *Set[T] {

	s := NewSet[T](entries...)
	s.Freeze()
	return s
}
