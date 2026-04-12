package correctness

import "sync"

const (
	managerStateBlobKey      = "correctness:manager"
	overlayStateBlobKeyPrefix = "correctness:overlay:"
)

type StateStore interface {
	GetBlob(key string) ([]byte, error)
	UpdateBlob(key string, fn func([]byte) ([]byte, error)) error
}

type memoryStateStore struct {
	mu        sync.Mutex
	blobs     map[string][]byte
	blobLocks sync.Map
}

func newMemoryStateStore() *memoryStateStore {
	return &memoryStateStore{
		blobs: make(map[string][]byte),
	}
}

func (m *memoryStateStore) GetBlob(key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	val := m.blobs[key]
	if val == nil {
		return nil, nil
	}
	return append([]byte(nil), val...), nil
}

func (m *memoryStateStore) blobLock(key string) *sync.Mutex {
	lock, _ := m.blobLocks.LoadOrStore(key, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

func (m *memoryStateStore) UpdateBlob(key string, fn func([]byte) ([]byte, error)) error {
	lock := m.blobLock(key)
	lock.Lock()
	defer lock.Unlock()

	m.mu.Lock()
	current := append([]byte(nil), m.blobs[key]...)
	m.mu.Unlock()
	next, err := fn(current)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if next == nil {
		delete(m.blobs, key)
		return nil
	}
	m.blobs[key] = append([]byte(nil), next...)
	return nil
}
