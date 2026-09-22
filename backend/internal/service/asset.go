package service

import (
	"context"
	"strings"

	"golang.org/x/sync/errgroup"

	"smart-grid-asset-management/backend/internal/base/errx"
	"smart-grid-asset-management/backend/internal/defined"
	"smart-grid-asset-management/backend/internal/entity/dto"
	"smart-grid-asset-management/backend/internal/entity/model"
	"smart-grid-asset-management/backend/internal/repository"
)

// AssetService answers the hierarchy, search, list and stats queries and
// performs the two deletes.
type AssetService struct {
	assets  *repository.AssetRepository
	lookups *LookupCache
}

func NewAssetService(assets *repository.AssetRepository, lookups *LookupCache) *AssetService {
	return &AssetService{assets: assets, lookups: lookups}
}

// Roots lists every top-level asset with its child count and subtree size.
func (s *AssetService) Roots(ctx context.Context) (dto.RootsResponse, error) {
	roots, err := s.assets.ListWithCounts(ctx, nil)
	if err != nil {
		return dto.RootsResponse{}, err
	}
	out := dto.RootsResponse{Total: len(roots), Roots: make([]dto.AssetNode, 0, len(roots))}
	for _, r := range roots {
		out.Roots = append(out.Roots, r.ToNode())
	}
	return out, nil
}

// Get returns one asset's details, or errx.ErrNotFound.
func (s *AssetService) Get(ctx context.Context, id string) (dto.AssetDetail, error) {
	a, err := s.assets.Get(ctx, id)
	if err != nil {
		return dto.AssetDetail{}, err
	}
	return a.ToDetail(), nil
}

// Children returns an asset's immediate children grouped by type, plus
// how many descendants of each type sit beneath it. The three queries
// involved are independent, so they run concurrently: existence check,
// children with their counts, and descendant counts.
func (s *AssetService) Children(ctx context.Context, id string) (dto.ChildrenResponse, error) {
	var (
		exists bool
		kids   []model.AssetWithCounts
		desc   []model.TypeCount
	)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() (err error) { exists, err = s.assets.Exists(gctx, id); return })
	g.Go(func() (err error) { kids, err = s.assets.ListWithCounts(gctx, &id); return })
	g.Go(func() (err error) { desc, err = s.assets.DescendantCounts(gctx, id); return })
	if err := g.Wait(); err != nil {
		return dto.ChildrenResponse{}, err
	}
	if !exists {
		return dto.ChildrenResponse{}, errx.ErrNotFound
	}

	// Rows arrive ordered by type, so each group is a contiguous run.
	groups := []dto.ChildGroup{}
	for _, k := range kids {
		node := k.ToNode()
		if n := len(groups); n == 0 || groups[n-1].AssetType != node.AssetType {
			groups = append(groups, dto.ChildGroup{AssetType: node.AssetType, Assets: []dto.AssetNode{}})
		}
		g := &groups[len(groups)-1]
		g.Count++
		g.SubtreeCount += node.SubtreeCount
		g.Assets = append(g.Assets, node)
	}
	counts := make([]dto.TypeCount, 0, len(desc))
	for _, d := range desc {
		counts = append(counts, d.ToDTO())
	}
	return dto.ChildrenResponse{AssetID: id, TotalChildren: len(kids), Groups: groups, DescendantCounts: counts}, nil
}

// Ancestors returns the path from the top of the hierarchy to the asset
// (the asset last), or errx.ErrNotFound.
func (s *AssetService) Ancestors(ctx context.Context, id string) (dto.AncestorsResponse, error) {
	path, err := s.assets.Ancestors(ctx, id)
	if err != nil {
		return dto.AncestorsResponse{}, err
	}
	out := dto.AncestorsResponse{AssetID: id, Path: make([]dto.AssetSummary, 0, len(path))}
	for _, a := range path {
		out.Path = append(out.Path, a.ToSummary())
	}
	return out, nil
}

// Search finds assets whose id or name contains req.Q, optionally of one
// type. Limit 0 means the default; problems with the parameters are
// *errx.InputError so the handler can answer 400.
func (s *AssetService) Search(ctx context.Context, req dto.SearchRequest) (dto.SearchResponse, error) {
	if err := req.Validate(); err != nil {
		return dto.SearchResponse{}, err
	}
	limit := req.Limit
	if limit == 0 {
		limit = defined.DefaultSearchSize
	}
	assetType := req.Type
	if assetType != "" {
		snap := s.lookups.Snapshot()
		canonical, ok := snap.MatchAssetType(assetType)
		if !ok {
			return dto.SearchResponse{}, errx.InvalidInput("unknown asset type %q (allowed: %s)", assetType, strings.Join(snap.AssetTypes(), ", "))
		}
		assetType = canonical
	}

	found, err := s.assets.Search(ctx, req.Q, assetType, limit+1) // one extra row tells us if results were cut off
	if err != nil {
		return dto.SearchResponse{}, err
	}
	truncated := len(found) > limit
	if truncated {
		found = found[:limit]
	}
	out := dto.SearchResponse{Query: req.Q, Type: assetType, Count: len(found), Truncated: truncated, Results: make([]dto.AssetSummary, 0, len(found))}
	for _, a := range found {
		out.Results = append(out.Results, a.ToSummary())
	}
	return out, nil
}

// List returns one page of assets, optionally filtered by text, type and
// status. Problems with the parameters are *errx.InputError.
func (s *AssetService) List(ctx context.Context, req dto.ListRequest) (dto.AssetListResponse, error) {
	if err := req.Validate(); err != nil {
		return dto.AssetListResponse{}, err
	}
	snap := s.lookups.Snapshot()
	if req.Type != "" {
		canonical, ok := snap.MatchAssetType(req.Type)
		if !ok {
			return dto.AssetListResponse{}, errx.InvalidInput("unknown asset type %q (allowed: %s)", req.Type, strings.Join(snap.AssetTypes(), ", "))
		}
		req.Type = canonical
	}
	if req.Status != "" {
		canonical, ok := snap.MatchStatus(req.Status)
		if !ok {
			return dto.AssetListResponse{}, errx.InvalidInput("unknown status %q (allowed: %s)", req.Status, strings.Join(snap.Statuses(), ", "))
		}
		req.Status = canonical
	}

	found, total, err := s.assets.List(ctx, repository.AssetFilter{
		Q: req.Q, Type: req.Type, Status: req.Status, Sort: req.Sort, Desc: req.Dir == defined.SortDesc,
		Limit: req.Limit, Offset: req.Offset,
	})
	if err != nil {
		return dto.AssetListResponse{}, err
	}
	out := dto.AssetListResponse{Total: int(total), Limit: req.Limit, Offset: req.Offset, Items: make([]dto.AssetSummary, 0, len(found))}
	for _, a := range found {
		out.Items = append(out.Items, a.ToSummary())
	}
	return out, nil
}

// Stats counts everything stored, by type and by status, for the overview.
func (s *AssetService) Stats(ctx context.Context) (dto.StatsResponse, error) {
	var (
		byType   []model.TypeCount
		byStatus []model.StatusCount
	)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() (err error) { byType, err = s.assets.CountByType(gctx); return })
	g.Go(func() (err error) { byStatus, err = s.assets.CountByStatus(gctx); return })
	if err := g.Wait(); err != nil {
		return dto.StatsResponse{}, err
	}
	out := dto.StatsResponse{ByType: make([]dto.TypeCount, 0, len(byType)), ByStatus: make([]dto.StatusCount, 0, len(byStatus))}
	for _, t := range byType {
		out.Total += t.Count
		out.ByType = append(out.ByType, t.ToDTO())
	}
	for _, st := range byStatus {
		out.ByStatus = append(out.ByStatus, st.ToDTO())
	}
	return out, nil
}

// Delete removes one asset that has no children: errx.ErrNotFound if there is
// no such asset, *errx.ConflictError if it still has children.
func (s *AssetService) Delete(ctx context.Context, id string) error {
	return s.assets.Delete(ctx, id)
}
