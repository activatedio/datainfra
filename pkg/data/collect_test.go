package data_test

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/activatedio/datainfra/pkg/data"
)

// labelled is a row type that carries labels so a selector can be applied.
type labelled struct {
	ID     int
	Labels data.Labels
}

func (l labelled) GetLabels() data.Labels { return l.Labels }

// pagedSource fakes a paged backend over n rows: tokens are the string form
// of the next offset, each page holds pageSize rows (or the backend default
// when Count <= 0), and a selector is applied after the page is cut, exactly
// as the gorm template does.
type pagedSource struct {
	n           int
	defaultSize int
	labelsFor   func(i int) data.Labels
	calls       []data.ListParams
}

func (s *pagedSource) list(_ context.Context, params data.ListParams) (*data.List[labelled], error) {
	s.calls = append(s.calls, params)

	offset := 0
	size := s.defaultSize
	if params.PageParams != nil {
		if params.PageParams.PageToken != "" {
			var err error
			offset, err = strconv.Atoi(params.PageParams.PageToken)
			if err != nil {
				return nil, err
			}
		}
		if params.PageParams.Count > 0 {
			size = params.PageParams.Count
		}
	}

	end := offset + size
	if end > s.n {
		end = s.n
	}
	rows := make([]labelled, 0, end-offset)
	for i := offset; i < end; i++ {
		var lbl data.Labels
		if s.labelsFor != nil {
			lbl = s.labelsFor(i)
		}
		rows = append(rows, labelled{ID: i, Labels: lbl})
	}

	if params.Selector != nil {
		var err error
		rows, err = data.FilterByLabels(params.Selector, rows)
		if err != nil {
			return nil, err
		}
	}

	next := ""
	if end < s.n {
		next = strconv.Itoa(end)
	}
	return &data.List[labelled]{NextPageToken: next, List: rows}, nil
}

func ids(rows []labelled) []int {
	out := make([]int, len(rows))
	for i, r := range rows {
		out[i] = r.ID
	}
	return out
}

func TestCollect_FollowsTokensToExhaustion(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	src := &pagedSource{n: 250, defaultSize: 100}
	got, err := data.Collect(context.Background(), src.list, data.CollectParams{})
	r.NoError(err)
	a.Len(got, 250)
	a.Equal(0, got[0].ID)
	a.Equal(249, got[249].ID)
	a.Len(src.calls, 3, "250 rows at the default size of 100 is three pages")
	a.Empty(src.calls[0].PageParams.PageToken)
	a.Equal("100", src.calls[1].PageParams.PageToken)
	a.Equal("200", src.calls[2].PageParams.PageToken)
}

func TestCollect_EmptyResult(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	src := &pagedSource{n: 0, defaultSize: 100}
	got, err := data.Collect(context.Background(), src.list, data.CollectParams{})
	r.NoError(err)
	a.Empty(got)
	a.Len(src.calls, 1)
}

func TestCollect_PassesPageSizeAndSelectorThrough(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	sel, err := labels.Parse("parity=even")
	r.NoError(err)

	src := &pagedSource{
		n:           10,
		defaultSize: 100,
		labelsFor: func(i int) data.Labels {
			if i%2 == 0 {
				return data.Labels{"parity": "even"}
			}
			return data.Labels{"parity": "odd"}
		},
	}
	got, err := data.Collect(context.Background(), src.list, data.CollectParams{PageSize: 3, Selector: sel})
	r.NoError(err)
	a.Equal([]int{0, 2, 4, 6, 8}, ids(got))
	a.Len(src.calls, 4, "10 rows at 3 per page is four pages")
	for _, c := range src.calls {
		a.Equal(3, c.PageParams.Count)
		a.Equal(sel, c.Selector)
	}
}

func TestCollect_CeilingDefaultsToTenThousand(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	a.Equal(10000, data.DefaultCollectMax)

	// Exactly the ceiling is allowed.
	src := &pagedSource{n: data.DefaultCollectMax, defaultSize: 1000}
	got, err := data.Collect(context.Background(), src.list, data.CollectParams{})
	r.NoError(err)
	a.Len(got, data.DefaultCollectMax)

	// One past it is not, and the caller gets nothing rather than a prefix.
	src = &pagedSource{n: data.DefaultCollectMax + 1, defaultSize: 1000}
	got, err = data.Collect(context.Background(), src.list, data.CollectParams{})
	r.Error(err)
	a.Nil(got)

	var limit data.CollectLimitExceeded
	r.ErrorAs(err, &limit)
	a.Equal(data.DefaultCollectMax, limit.Max)
	a.Greater(limit.Seen, limit.Max)
}

func TestCollect_CeilingErrorNamesTheLever(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	src := &pagedSource{n: 50, defaultSize: 100}
	_, err := data.Collect(context.Background(), src.list, data.CollectParams{Max: 20})
	r.Error(err)

	var limit data.CollectLimitExceeded
	r.ErrorAs(err, &limit)
	a.Equal(20, limit.Max)
	a.Equal(50, limit.Seen)

	// Whoever hits the ceiling is rarely whoever chose it: the message must
	// say what the ceiling was, how far it got, and that it is adjustable.
	msg := err.Error()
	a.Contains(msg, "20 rows")
	a.Contains(msg, "50")
	a.Contains(msg, "CollectParams.Max")
	a.Contains(msg, fmt.Sprint(data.DefaultCollectMax))
	a.Contains(msg, "raise")
	a.Contains(msg, "ExistsAny")
}

func TestCollect_BoundedMemoryUnderCeiling(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	// A million-row source with a ceiling of 250 stops after the page that
	// crosses the ceiling; it never walks the whole table.
	src := &pagedSource{n: 1_000_000, defaultSize: 100}
	_, err := data.Collect(context.Background(), src.list, data.CollectParams{Max: 250})
	r.Error(err)
	a.Len(src.calls, 3, "should stop on the page that crosses 250")
}

func TestCollect_NonAdvancingTokenIsAnError(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	stuck := func(_ context.Context, _ data.ListParams) (*data.List[labelled], error) {
		return &data.List[labelled]{NextPageToken: "same", List: []labelled{{ID: 1}}}, nil
	}
	_, err := data.Collect(context.Background(), stuck, data.CollectParams{})
	r.Error(err)
	a.Contains(err.Error(), "did not advance")
}

func TestCollect_PropagatesListError(t *testing.T) {
	r := require.New(t)

	boom := errors.New("boom")
	failing := func(_ context.Context, _ data.ListParams) (*data.List[labelled], error) {
		return nil, boom
	}
	_, err := data.Collect(context.Background(), failing, data.CollectParams{})
	r.ErrorIs(err, boom)
}

func TestExistsAny_WithoutSelectorAsksForOneRow(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	src := &pagedSource{n: 1_000_000, defaultSize: 100}
	ok, err := data.ExistsAny(context.Background(), src.list, nil)
	r.NoError(err)
	a.True(ok)
	r.Len(src.calls, 1)
	a.Equal(1, src.calls[0].PageParams.Count)

	empty := &pagedSource{n: 0, defaultSize: 100}
	ok, err = data.ExistsAny(context.Background(), empty.list, nil)
	r.NoError(err)
	a.False(ok)
}

func TestExistsAny_WithSelectorWalksFilteredPages(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	sel, err := labels.Parse("tier=gold")
	r.NoError(err)

	// Only row 250 is gold: the first two default-size pages filter to
	// nothing but still carry a token, and ExistsAny must keep going.
	gold := func(i int) data.Labels {
		if i == 250 {
			return data.Labels{"tier": "gold"}
		}
		return data.Labels{"tier": "bronze"}
	}
	src := &pagedSource{n: 300, defaultSize: 100, labelsFor: gold}
	ok, err := data.ExistsAny(context.Background(), src.list, sel)
	r.NoError(err)
	a.True(ok)
	a.Len(src.calls, 3)

	// No gold anywhere: walks to the end and says no.
	src = &pagedSource{n: 300, defaultSize: 100, labelsFor: func(int) data.Labels { return data.Labels{"tier": "bronze"} }}
	ok, err = data.ExistsAny(context.Background(), src.list, sel)
	r.NoError(err)
	a.False(ok)
	a.Len(src.calls, 3)
}

func TestExistsAny_AgreesWithCollectEmptiness(t *testing.T) {
	r := require.New(t)

	sel, err := labels.Parse("tier=gold")
	r.NoError(err)

	cases := []struct {
		name string
		n    int
		gold func(i int) bool
	}{
		{"empty", 0, func(int) bool { return false }},
		{"rows, none gold", 120, func(int) bool { return false }},
		{"rows, first gold", 120, func(i int) bool { return i == 0 }},
		{"rows, last gold", 120, func(i int) bool { return i == 119 }},
		{"rows, all gold", 120, func(int) bool { return true }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)
			labelsFor := func(i int) data.Labels {
				if tc.gold(i) {
					return data.Labels{"tier": "gold"}
				}
				return data.Labels{"tier": "bronze"}
			}
			for _, s := range []labels.Selector{nil, sel} {
				src := &pagedSource{n: tc.n, defaultSize: 50, labelsFor: labelsFor}
				all, err := data.Collect(context.Background(), src.list, data.CollectParams{Selector: s})
				r.NoError(err)
				ok, err := data.ExistsAny(context.Background(), src.list, s)
				r.NoError(err)
				a.Equal(len(all) > 0, ok, "selector=%v", s)
			}
		})
	}
}

func TestExistsAny_NonAdvancingTokenIsAnError(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	stuck := func(_ context.Context, _ data.ListParams) (*data.List[labelled], error) {
		return &data.List[labelled]{NextPageToken: "same"}, nil
	}
	_, err := data.ExistsAny(context.Background(), stuck, nil)
	r.Error(err)
	a.Contains(err.Error(), "did not advance")
}
