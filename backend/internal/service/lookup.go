package service

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync/atomic"

	"golang.org/x/sync/errgroup"

	"smart-grid-asset-management/backend/internal/entity/dto"
	"smart-grid-asset-management/backend/internal/entity/model"
)

// LookupSource loads the reference tables (satisfied by
// *repository.LookupRepository).
type LookupSource interface {
	AssetTypes(ctx context.Context) ([]string, error)
	OperationalStatuses(ctx context.Context) ([]string, error)
	ParentRules(ctx context.Context) ([]model.ParentRule, error)
}

// LookupSnapshot is an immutable, in-memory copy of asset_types,
// operational_statuses and asset_type_parent_rules. Validation reads a
// snapshot — never the database per row, and never hardcoded constants —
// so adding a type, status or parent pairing is a data change. All methods
// are safe for concurrent use; the slices they return must not be modified.
type LookupSnapshot struct {
	assetTypes []string
	statuses   []string
	typeByFold map[string]string // lower-cased code -> canonical code
	statusFold map[string]string
	parents    map[string][]string // child type -> allowed parent types, sorted
	rules      []model.ParentRule
	roots      []string
}

func newLookupSnapshot(types, statuses []string, rules []model.ParentRule) *LookupSnapshot {
	s := &LookupSnapshot{
		assetTypes: sortedCopy(types),
		statuses:   sortedCopy(statuses),
		typeByFold: make(map[string]string, len(types)),
		statusFold: make(map[string]string, len(statuses)),
		parents:    make(map[string][]string),
		rules:      slices.Clone(rules),
	}
	for _, t := range s.assetTypes {
		s.typeByFold[strings.ToLower(t)] = t
	}
	for _, st := range s.statuses {
		s.statusFold[strings.ToLower(st)] = st
	}
	for _, r := range s.rules {
		s.parents[r.ChildType] = append(s.parents[r.ChildType], r.ParentType)
	}
	for child := range s.parents {
		slices.Sort(s.parents[child])
	}
	for _, t := range s.assetTypes {
		if len(s.parents[t]) == 0 {
			s.roots = append(s.roots, t)
		}
	}
	return s
}

func sortedCopy(in []string) []string {
	out := slices.Clone(in)
	slices.Sort(out)
	return out
}

// MatchAssetType resolves raw to a canonical asset type code,
// case-insensitively.
func (s *LookupSnapshot) MatchAssetType(raw string) (string, bool) {
	c, ok := s.typeByFold[strings.ToLower(raw)]
	return c, ok
}

// MatchStatus resolves raw to a canonical operational status code,
// case-insensitively.
func (s *LookupSnapshot) MatchStatus(raw string) (string, bool) {
	c, ok := s.statusFold[strings.ToLower(raw)]
	return c, ok
}

// HasAssetType reports whether code is exactly a known asset type.
func (s *LookupSnapshot) HasAssetType(code string) bool {
	c, ok := s.typeByFold[strings.ToLower(code)]
	return ok && c == code
}

// AllowedParents lists the types a child of the given type may sit under.
func (s *LookupSnapshot) AllowedParents(childType string) []string { return s.parents[childType] }

// IsRootType reports whether the type is known and has no parent rule —
// such assets sit at the top of the hierarchy and must have no parent.
func (s *LookupSnapshot) IsRootType(t string) bool {
	return s.HasAssetType(t) && len(s.parents[t]) == 0
}

func (s *LookupSnapshot) AssetTypes() []string            { return s.assetTypes }
func (s *LookupSnapshot) Statuses() []string              { return s.statuses }
func (s *LookupSnapshot) RootTypes() []string             { return s.roots }
func (s *LookupSnapshot) ParentRules() []model.ParentRule { return s.rules }

// LookupsResponse renders the snapshot for GET /api/lookups.
func (s *LookupSnapshot) LookupsResponse() dto.LookupsResponse {
	rules := make([]dto.ParentRule, 0, len(s.rules))
	for _, r := range s.rules {
		rules = append(rules, r.ToDTO())
	}
	return dto.LookupsResponse{
		AssetTypes:          nonNil(s.assetTypes),
		OperationalStatuses: nonNil(s.statuses),
		ParentRules:         rules,
		RootTypes:           nonNil(s.roots),
	}
}

// nonNil keeps empty lists serialising as [] rather than null.
func nonNil(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

// LookupCache holds the current LookupSnapshot and swaps it atomically on
// Refresh, so concurrent readers always see one consistent snapshot.
type LookupCache struct {
	src LookupSource
	cur atomic.Pointer[LookupSnapshot]
}

// NewLookupCache returns a cache with an empty snapshot; call Refresh
// before serving traffic.
func NewLookupCache(src LookupSource) *LookupCache {
	c := &LookupCache{src: src}
	c.cur.Store(newLookupSnapshot(nil, nil, nil))
	return c
}

// Snapshot returns the current snapshot. A validation run should take it
// once and use it throughout so it never straddles a refresh.
func (c *LookupCache) Snapshot() *LookupSnapshot { return c.cur.Load() }

// Refresh reloads the three reference tables concurrently and swaps the
// snapshot in. On error the previous snapshot stays in place.
func (c *LookupCache) Refresh(ctx context.Context) error {
	var (
		types, statuses []string
		rules           []model.ParentRule
	)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() (err error) { types, err = c.src.AssetTypes(gctx); return })
	g.Go(func() (err error) { statuses, err = c.src.OperationalStatuses(gctx); return })
	g.Go(func() (err error) { rules, err = c.src.ParentRules(gctx); return })
	if err := g.Wait(); err != nil {
		return fmt.Errorf("refresh lookup tables: %w", err)
	}
	c.cur.Store(newLookupSnapshot(types, statuses, rules))
	return nil
}
