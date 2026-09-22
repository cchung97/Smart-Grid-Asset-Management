package service

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"

	"smart-grid-asset-management/backend/internal/entity/model"
)

func TestLookupSnapshot(t *testing.T) {
	s := testSnapshot()

	if got := s.RootTypes(); !reflect.DeepEqual(got, []string{"SUBSTATION"}) {
		t.Errorf("RootTypes() = %v, want [SUBSTATION] (derived: the type with no parent rule)", got)
	}
	if !s.IsRootType("SUBSTATION") || s.IsRootType("TRANSFORMER") || s.IsRootType("PYLON") {
		t.Error("IsRootType must be true only for known types without a parent rule")
	}
	if got := s.AllowedParents("SWITCHBOARD_PANEL"); !reflect.DeepEqual(got, []string{"SWITCHBOARD"}) {
		t.Errorf("AllowedParents(SWITCHBOARD_PANEL) = %v", got)
	}
	if c, ok := s.MatchAssetType("lv_board"); !ok || c != "LV_BOARD" {
		t.Errorf("MatchAssetType(lv_board) = %q, %v; want canonical LV_BOARD", c, ok)
	}
	if _, ok := s.MatchStatus("nope"); ok {
		t.Error("MatchStatus(nope) = true")
	}
	if !s.HasAssetType("SUBSTATION") || s.HasAssetType("substation") {
		t.Error("HasAssetType is an exact-code check")
	}
}

type fakeLookups struct {
	mu       sync.Mutex
	types    []string
	statuses []string
	rules    []model.ParentRule
	err      error
}

func (f *fakeLookups) get() ([]string, []string, []model.ParentRule, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.types, f.statuses, f.rules, f.err
}
func (f *fakeLookups) AssetTypes(context.Context) ([]string, error) {
	ty, _, _, err := f.get()
	return ty, err
}
func (f *fakeLookups) OperationalStatuses(context.Context) ([]string, error) {
	_, st, _, err := f.get()
	return st, err
}
func (f *fakeLookups) ParentRules(context.Context) ([]model.ParentRule, error) {
	_, _, r, err := f.get()
	return r, err
}

func TestLookupCache_RefreshSwapsSnapshotAndKeepsOldOnError(t *testing.T) {
	src := &fakeLookups{types: []string{"A"}, statuses: []string{"OK"}}
	cache := NewLookupCache(src)
	if got := cache.Snapshot().AssetTypes(); len(got) != 0 {
		t.Fatalf("snapshot before Refresh = %v, want empty", got)
	}

	if err := cache.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	old := cache.Snapshot()
	if !old.HasAssetType("A") {
		t.Fatal("Refresh did not load asset types")
	}

	src.mu.Lock()
	src.types = []string{"A", "B"} // a new type appears in the table: no code change needed
	src.mu.Unlock()
	if err := cache.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !cache.Snapshot().HasAssetType("B") {
		t.Error("new asset type not visible after Refresh")
	}
	if old.HasAssetType("B") {
		t.Error("a previously taken snapshot must be immutable")
	}

	boom := errors.New("db down")
	src.mu.Lock()
	src.err = boom
	src.mu.Unlock()
	if err := cache.Refresh(context.Background()); !errors.Is(err, boom) {
		t.Fatalf("Refresh error = %v, want wrapped db error", err)
	}
	if !cache.Snapshot().HasAssetType("B") {
		t.Error("a failed Refresh must leave the previous snapshot in place")
	}
}

func TestLookupCache_ConcurrentReadersAndRefreshes(t *testing.T) {
	src := &fakeLookups{types: []string{"A"}, statuses: []string{"OK"}}
	cache := NewLookupCache(src)
	_ = cache.Refresh(context.Background())

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				_ = cache.Refresh(context.Background())
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				s := cache.Snapshot()
				_ = s.HasAssetType("A")
				_ = s.IsRootType("A")
			}
		}()
	}
	wg.Wait()
}
