package fault_catalog

import (
	"context"
	"testing"
)

func TestLoadCatalog(t *testing.T) {
	if err := LoadCatalog("testdata"); err != nil {
		t.Fatalf("LoadCatalog failed: %v", err)
	}

	svc := NewService(nil)
	faults := svc.ListFaults("", "", "")
	if len(faults) < 1 {
		t.Errorf("expected at least 1 fault, got %d", len(faults))
	}
}

func TestGetFault(t *testing.T) {
	if err := LoadCatalog("testdata"); err != nil {
		t.Fatalf("LoadCatalog failed: %v", err)
	}

	svc := NewService(nil)
	entry, err := svc.GetFault("pod-delete")
	if err != nil {
		t.Fatalf("GetFault(pod-delete) returned error: %v", err)
	}
	if entry == nil {
		t.Fatal("GetFault returned nil entry")
	}
	if entry.Metadata.Name != "pod-delete" {
		t.Errorf("expected name pod-delete, got %s", entry.Metadata.Name)
	}
}

func TestGetFaultNotFound(t *testing.T) {
	if err := LoadCatalog("testdata"); err != nil {
		t.Fatalf("LoadCatalog failed: %v", err)
	}

	svc := NewService(nil)
	_, err := svc.GetFault("nonexistent-fault")
	if err == nil {
		t.Fatal("expected error for nonexistent fault, got nil")
	}
	if _, ok := err.(ErrFaultNotFound); !ok {
		t.Errorf("expected ErrFaultNotFound, got %T: %v", err, err)
	}
}

func TestFaultsForApp_NoAppCatalog(t *testing.T) {
	if err := LoadCatalog("testdata"); err != nil {
		t.Fatalf("LoadCatalog failed: %v", err)
	}

	svc := NewService(nil)
	faults, err := svc.FaultsForApp(context.Background(), "some-app")
	if err != nil {
		t.Fatalf("FaultsForApp returned error: %v", err)
	}
	// Should return at least general faults
	if len(faults) < 1 {
		t.Errorf("expected at least 1 fault for any app, got %d", len(faults))
	}
}
