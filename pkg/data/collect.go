package data

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/labels"
)

// ListFunc is the shape shared by every paged list method a repository
// exposes — ListAll, ListBy<Association>, and hand-written variants such as
// ListActive. A method value like repo.ListAll satisfies it directly.
type ListFunc[E any] func(ctx context.Context, params ListParams) (*List[E], error)

// DefaultCollectMax is the ceiling Collect applies when CollectParams.Max is
// zero or negative.
const DefaultCollectMax = 10000

// CollectParams configures Collect.
type CollectParams struct {
	// Max is the most rows Collect will hold before giving up with
	// CollectLimitExceeded. Zero or negative means DefaultCollectMax. Choose
	// it for what the caller can afford to hold in memory, not for what the
	// table is expected to contain today.
	Max int
	// PageSize is the Count requested per page. Zero means the backend's
	// default page size.
	PageSize int
	// Selector narrows the result by labels, applied by the backend to each
	// page.
	Selector labels.Selector
}

// CollectLimitExceeded is returned by Collect when the result would exceed
// CollectParams.Max rows. No partial result is returned with it.
type CollectLimitExceeded struct {
	// Max is the ceiling that was in force.
	Max int
	// Seen is how many rows had been collected when Collect stopped; it is
	// always greater than Max.
	Seen int
}

func (e CollectLimitExceeded) Error() string {
	return fmt.Sprintf("collect: result exceeds the ceiling of %d rows (stopped after collecting %d); "+
		"the ceiling is CollectParams.Max (default %d) and is the caller's to raise if it can hold that many rows, "+
		"otherwise page with ListParams.PageParams or test with ExistsAny instead",
		e.Max, e.Seen, DefaultCollectMax)
}

// Collect follows NextPageToken from list until the result is complete and
// returns every row in one slice. It is the replacement for calling a list
// method once with an empty ListParams, which only ever returned the first
// page.
//
// Collect never truncates. If more than params.Max rows are matched it
// returns CollectLimitExceeded and nil, so the caller either raises the
// ceiling deliberately or restructures the read. Rows held in memory never
// exceed Max plus one page.
func Collect[E any](ctx context.Context, list ListFunc[E], params CollectParams) ([]E, error) {

	limit := params.Max
	if limit <= 0 {
		limit = DefaultCollectMax
	}

	var out []E
	token := ""
	for {
		page, err := list(ctx, ListParams{
			PageParams: &PageParams{PageToken: token, Count: params.PageSize},
			Selector:   params.Selector,
		})
		if err != nil {
			return nil, err
		}
		if page == nil {
			return nil, errors.New("collect: list returned a nil page")
		}
		out = append(out, page.List...)
		if len(out) > limit {
			return nil, CollectLimitExceeded{Max: limit, Seen: len(out)}
		}
		if page.NextPageToken == "" {
			return out, nil
		}
		if page.NextPageToken == token {
			return nil, fmt.Errorf("collect: page token %q did not advance; the list function is not paging", token)
		}
		token = page.NextPageToken
	}
}

// ExistsAny reports whether list yields at least one row under the ambient
// context scope, narrowed by selector when one is given. It answers the
// question "is there anything here?" without collecting, so it is the right
// primitive for emptiness guards over unbounded populations — the place
// where Collect would turn a correct answer into CollectLimitExceeded.
//
// Without a selector it asks for a single row. With a selector the backend
// filters each page after fetching it, so ExistsAny follows page tokens
// past filtered-out pages until a row matches or the result ends; it asks
// for default-size pages there to keep the walk short. Either way its
// answer agrees with whether Collect over the same list and selector would
// return an empty slice.
func ExistsAny[E any](ctx context.Context, list ListFunc[E], selector labels.Selector) (bool, error) {

	count := 1
	if selector != nil {
		count = 0
	}

	token := ""
	for {
		page, err := list(ctx, ListParams{
			PageParams: &PageParams{PageToken: token, Count: count},
			Selector:   selector,
		})
		if err != nil {
			return false, err
		}
		if page == nil {
			return false, errors.New("exists: list returned a nil page")
		}
		if len(page.List) > 0 {
			return true, nil
		}
		if page.NextPageToken == "" {
			return false, nil
		}
		if page.NextPageToken == token {
			return false, fmt.Errorf("exists: page token %q did not advance; the list function is not paging", token)
		}
		token = page.NextPageToken
	}
}
