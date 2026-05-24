package metadata

import (
	"context"
	"testing"
)

func TestFederatedCrewStore_PublishAndFetch(t *testing.T) {
	ctx := context.Background()
	store := NewMockRegistryStore()

	// Initial fetch should return nil
	info, err := store.FetchPackage(ctx, "tenant-a", "researcher-crew", "v1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info != nil {
		t.Fatal("expected package info to be nil initially")
	}

	// Publish package
	err = store.PublishPackage(ctx, "tenant-a", "researcher-crew", "v1.0.0", "sha256-abc12345", 5000, "storage-key-1", []byte(`{"type":"crew"}`), "operator-1")
	if err != nil {
		t.Fatalf("failed to publish package: %v", err)
	}

	// Fetch package again
	info, err = store.FetchPackage(ctx, "tenant-a", "researcher-crew", "v1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info == nil {
		t.Fatal("expected package info to be found")
	}

	if info.Name != "researcher-crew" {
		t.Errorf("expected name 'researcher-crew', got: %s", info.Name)
	}
	if info.Digest != "sha256-abc12345" {
		t.Errorf("expected digest 'sha256-abc12345', got: %s", info.Digest)
	}
	if info.PublishedBy != "operator-1" {
		t.Errorf("expected published by 'operator-1', got: %s", info.PublishedBy)
	}
}

func TestFederatedCrewStore_ListScope(t *testing.T) {
	ctx := context.Background()
	store := NewMockRegistryStore()

	// Publish in Tenant A
	_ = store.PublishPackage(ctx, "tenant-a", "agent-a", "v1.0.0", "sha-a", 100, "key-a", []byte("{}"), "user-a")

	// Publish in Tenant B
	_ = store.PublishPackage(ctx, "tenant-b", "agent-b", "v1.1.0", "sha-b", 200, "key-b", []byte("{}"), "user-b")

	// List Tenant A packages
	listA, err := store.ListPackages(ctx, "tenant-a")
	if err != nil {
		t.Fatalf("failed to list tenant A: %v", err)
	}
	if len(listA) != 1 {
		t.Fatalf("expected 1 package in tenant A, got: %d", len(listA))
	}
	if listA[0].Name != "agent-a" {
		t.Errorf("expected agent-a in tenant A list, got: %s", listA[0].Name)
	}

	// List Tenant B packages
	listB, err := store.ListPackages(ctx, "tenant-b")
	if err != nil {
		t.Fatalf("failed to list tenant B: %v", err)
	}
	if len(listB) != 1 {
		t.Fatalf("expected 1 package in tenant B, got: %d", len(listB))
	}
	if listB[0].Name != "agent-b" {
		t.Errorf("expected agent-b in tenant B list, got: %s", listB[0].Name)
	}
}
