package dto

import (
	"errors"
	"strings"
	"testing"

	"smart-grid-asset-management/backend/internal/base/errx"
)

func TestSearchRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		in      SearchRequest
		wantErr string // substring of the InputError message; "" means valid
		wantQ   string // Q after normalisation, checked when valid
	}{
		{"minimal", SearchRequest{Q: "tx-001"}, "", "tx-001"},
		{"trims q and type", SearchRequest{Q: "  tx  ", Type: " TRANSFORMER "}, "", "tx"},
		{"limit at the maximum", SearchRequest{Q: "x", Limit: 200}, "", "x"},
		{"limit zero means default", SearchRequest{Q: "x", Limit: 0}, "", "x"},
		{"q missing", SearchRequest{}, "query parameter q is required", ""},
		{"q only whitespace", SearchRequest{Q: "   "}, "query parameter q is required", ""},
		{"q at the length cap", SearchRequest{Q: strings.Repeat("é", 200)}, "", strings.Repeat("é", 200)},
		{"q over the length cap counts runes", SearchRequest{Q: strings.Repeat("é", 201)}, "at most 200 characters", ""},
		{"limit negative", SearchRequest{Q: "x", Limit: -1}, "limit must be between 1 and 200", ""},
		{"limit too large", SearchRequest{Q: "x", Limit: 201}, "limit must be between 1 and 200", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := tc.in
			err := req.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				if req.Q != tc.wantQ {
					t.Errorf("Q = %q, want %q", req.Q, tc.wantQ)
				}
				return
			}
			var ie *errx.InputError
			if !errors.As(err, &ie) || !strings.Contains(ie.Msg, tc.wantErr) {
				t.Fatalf("Validate() = %v, want *errx.InputError containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestSearchRequest_Validate_TrimsTypeInPlace(t *testing.T) {
	req := SearchRequest{Q: "x", Type: "  LV_BOARD "}
	if err := req.Validate(); err != nil || req.Type != "LV_BOARD" {
		t.Errorf("Validate() = %v, Type = %q; want nil and trimmed LV_BOARD", err, req.Type)
	}
}

func TestImportIDRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{"uuid", "0b5f3c7e-8a52-4a55-9b0e-0c9f8a3f6d11", false},
		{"nil uuid", "00000000-0000-0000-0000-000000000000", false},
		{"empty", "", true},
		{"not a uuid", "bad", true},
		{"truncated", "0b5f3c7e-8a52-4a55-9b0e", true},
	}
	for _, tc := range tests {
		err := ImportIDRequest{ImportID: tc.id}.Validate()
		var ie *errx.InputError
		if tc.wantErr != (errors.As(err, &ie)) || (!tc.wantErr && err != nil) {
			t.Errorf("%s: Validate(%q) = %v, wantErr %v", tc.name, tc.id, err, tc.wantErr)
		}
	}
}

func TestListRequest_Validate(t *testing.T) {
	tests := []struct {
		name      string
		in        ListRequest
		wantErr   string
		wantLimit int
	}{
		{"defaults", ListRequest{}, "", 25},
		{"filters trimmed", ListRequest{Q: " tx ", Type: " X ", Status: " Y "}, "", 25},
		{"limit at the maximum", ListRequest{Limit: 200}, "", 200},
		{"limit too large", ListRequest{Limit: 201}, "limit must be between 1 and 200", 0},
		{"limit negative", ListRequest{Limit: -1}, "limit must be between 1 and 200", 0},
		{"offset negative", ListRequest{Offset: -5}, "offset must not be negative", 0},
		{"q over the cap", ListRequest{Q: strings.Repeat("x", 201)}, "at most 200 characters", 0},
		{"every sort key is accepted", ListRequest{Sort: "parent", Dir: "desc"}, "", 25},
		{"sort and dir are case-insensitive", ListRequest{Sort: " Name ", Dir: "DESC"}, "", 25},
		{"unknown sort", ListRequest{Sort: "rating"}, "sort must be one of: asset_id, name, type, status, parent", 0},
		{"sql in sort", ListRequest{Sort: "asset_id; drop table assets"}, "sort must be one of", 0},
		{"unknown direction", ListRequest{Dir: "up"}, "dir must be asc or desc", 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := tc.in
			err := req.Validate()
			if tc.wantErr == "" {
				if err != nil || req.Limit != tc.wantLimit || req.Q != strings.TrimSpace(tc.in.Q) {
					t.Fatalf("Validate() = %v, limit %d, q %q", err, req.Limit, req.Q)
				}
				return
			}
			var ie *errx.InputError
			if !errors.As(err, &ie) || !strings.Contains(ie.Msg, tc.wantErr) {
				t.Fatalf("Validate() = %v, want *InputError containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestImportListRequest_Validate(t *testing.T) {
	for name, tc := range map[string]struct {
		in      ImportListRequest
		wantErr bool
		limit   int
	}{
		"defaults":     {ImportListRequest{}, false, 25},
		"maximum":      {ImportListRequest{Limit: 100}, false, 100},
		"over maximum": {ImportListRequest{Limit: 101}, true, 0},
		"negative":     {ImportListRequest{Offset: -1}, true, 0},
	} {
		req := tc.in
		err := req.Validate()
		if (err != nil) != tc.wantErr || (!tc.wantErr && req.Limit != tc.limit) {
			t.Errorf("%s: Validate() = %v, limit %d", name, err, req.Limit)
		}
	}
}

func TestListRequest_Validate_DefaultsAndNormalisesSort(t *testing.T) {
	req := ListRequest{}
	if err := req.Validate(); err != nil || req.Sort != "asset_id" || req.Dir != "asc" {
		t.Fatalf("defaults: %v, sort %q, dir %q", err, req.Sort, req.Dir)
	}
	req = ListRequest{Sort: " Name ", Dir: "DESC"}
	if err := req.Validate(); err != nil || req.Sort != "name" || req.Dir != "desc" {
		t.Fatalf("normalised: %v, sort %q, dir %q", err, req.Sort, req.Dir)
	}
}
