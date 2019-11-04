package observe

import (
	"sync"

	"github.com/nogproject/nog/backend/pkg/ulid"
)

type VolatileStateStore struct {
	mu   sync.Mutex
	data map[string]ulid.I
}

func NewVolatileStateStore() *VolatileStateStore {
	return &VolatileStateStore{
		data: make(map[string]ulid.I),
	}
}

func (s *VolatileStateStore) LoadULID(name string) (ulid.I, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.data[name]
	if !ok {
		id = ulid.Nil
	}
	return id, nil
}

func (s *VolatileStateStore) SaveULID(name string, id ulid.I) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[name] = id
	return nil
}
