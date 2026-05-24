package metadata

import (
	"context"
	"sync"
)

type MockRegistryStore struct {
	mu    sync.RWMutex
	store map[string]map[string]map[string]RegistryPackageInfo // tenantID -> name -> version -> info
}

func NewMockRegistryStore() *MockRegistryStore {
	return &MockRegistryStore{
		store: make(map[string]map[string]map[string]RegistryPackageInfo),
	}
}

func (m *MockRegistryStore) PublishPackage(ctx context.Context, tenantID string, name string, version string, digest string, sizeBytes int64, storageKey string, manifest []byte, publishedBy string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.store[tenantID]; !ok {
		m.store[tenantID] = make(map[string]map[string]RegistryPackageInfo)
	}
	if _, ok := m.store[tenantID][name]; !ok {
		m.store[tenantID][name] = make(map[string]RegistryPackageInfo)
	}

	m.store[tenantID][name][version] = RegistryPackageInfo{
		Name:        name,
		Version:     version,
		Digest:      digest,
		SizeBytes:   sizeBytes,
		PublishedBy: publishedBy,
	}

	return nil
}

func (m *MockRegistryStore) FetchPackage(ctx context.Context, tenantID string, name string, version string) (*RegistryPackageInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tenant, ok := m.store[tenantID]
	if !ok {
		return nil, nil
	}
	pkg, ok := tenant[name]
	if !ok {
		return nil, nil
	}
	info, ok := pkg[version]
	if !ok {
		return nil, nil
	}

	return &info, nil
}

func (m *MockRegistryStore) ListPackages(ctx context.Context, tenantID string) ([]RegistryPackageInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := []RegistryPackageInfo{}
	tenant, ok := m.store[tenantID]
	if !ok {
		return results, nil
	}

	for _, pkg := range tenant {
		for _, info := range pkg {
			results = append(results, info)
		}
	}

	return results, nil
}
