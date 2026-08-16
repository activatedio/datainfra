package gorm

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/activatedio/datainfra/pkg/data"
)

// MappingHistogramTemplateParams configures a Postgres-only histogram
// template. Bindings should be the same slice passed to
// NewMappingSearchTemplate so the histogram applies an identical
// predicate-filter set to Search. DateColumn is the SQL column the date
// histogram aggregates on.
type MappingHistogramTemplateParams[E any, I any] struct {
	Template   MappingTemplate[E, I]
	Bindings   []SearchPredicateBinding
	DateColumn string
}

// NewMappingHistogramTemplate returns a data.SearchHistogramTemplate that
// computes date-histogram bucket counts on the Postgres backend. Other
// dialects (notably SQLite) return an error from SearchHistogram; the
// caller is expected to gate the feature on a Postgres deployment.
func NewMappingHistogramTemplate[E any, I any](params MappingHistogramTemplateParams[E, I]) data.SearchHistogramTemplate[E] {
	bindingsMap := make(map[string]SearchPredicateBinding, len(params.Bindings))
	for _, b := range params.Bindings {
		bindingsMap[b.Descriptor.Name] = b
	}
	return &histogramTemplateImpl[E, I]{
		template:    params.Template,
		bindings:    params.Bindings,
		bindingsMap: bindingsMap,
		dateColumn:  params.DateColumn,
	}
}

type histogramTemplateImpl[E any, I any] struct {
	template    MappingTemplate[E, I]
	bindings    []SearchPredicateBinding
	bindingsMap map[string]SearchPredicateBinding
	dateColumn  string
}

type histogramRow struct {
	Bucket time.Time
	Count  int64
}

// SearchHistogram returns one bucket count per slice across [Min, Max].
//
// The window is constrained inclusively on both ends — a bucket containing
// Max is included even when Max falls mid-bucket. Empty buckets at the
// edges are returned with Count=0 via a LEFT JOIN against a
// generate_series-driven bucket table. Predicate filters are applied to
// the events subquery using the same SearchPredicateBinding set that
// drives Search, so the histogram counts a row iff Search would have
// returned it.
func (c *histogramTemplateImpl[E, I]) SearchHistogram(
	ctx context.Context,
	criteria []*data.SearchPredicate,
	spec *data.HistogramSpec,
) (*data.HistogramResult, error) {

	if spec == nil {
		return nil, fmt.Errorf("histogram spec is required")
	}
	if c.dateColumn == "" {
		return nil, fmt.Errorf("histogram date column is not configured")
	}

	db, err := GetDB(ctx)
	if err != nil {
		return nil, err
	}

	if db.Name() != "postgres" {
		return nil, fmt.Errorf("SearchHistogram requires the Postgres backend; %s backend does not support date-histogram aggregations", db.Name())
	}

	resolved := spec.Interval
	if resolved.Unit == data.HistogramIntervalAuto {
		resolved = data.ResolveAutoInterval(spec.Min, spec.Max, 0)
	}

	truncExpr, intervalSQL, err := postgresBucketSpec(resolved, c.dateColumn)
	if err != nil {
		return nil, err
	}

	inner := db.Table(c.template.GetTable())
	inner = c.template.ApplyContextScopeQueryBuilder(ctx, inner, data.FetchTypeList)
	inner = inner.Where(fmt.Sprintf("%s >= ?", c.dateColumn), spec.Min).
		Where(fmt.Sprintf("%s <= ?", c.dateColumn), spec.Max)

	inner, err = c.applyCriteria(inner, criteria)
	if err != nil {
		return nil, err
	}
	inner = inner.Select(fmt.Sprintf("%s AS bucket_key", truncExpr))

	// generate_series anchors at the truncated lower bound so bucket
	// starts align to natural boundaries (calendar boundaries for
	// day/week/month; step-aligned for fixed sub-day widths). It
	// extends through Max so the bucket containing Max is included.
	seriesAnchor := postgresAlignExpr(resolved, "?::timestamptz")
	sql := "WITH buckets AS (SELECT generate_series(" + seriesAnchor + ", ?::timestamptz, ?::interval) AS bucket) " +
		"SELECT b.bucket AS bucket, COUNT(e.bucket_key) AS count " +
		"FROM buckets b LEFT JOIN (?) AS e ON e.bucket_key = b.bucket " +
		"GROUP BY b.bucket ORDER BY b.bucket"

	var rows []histogramRow
	if err := db.Raw(sql, spec.Min, spec.Max, intervalSQL, inner).Scan(&rows).Error; err != nil {
		return nil, err
	}

	buckets := make([]*data.HistogramBucket, len(rows))
	for i, r := range rows {
		buckets[i] = &data.HistogramBucket{Key: r.Bucket.UTC(), Count: r.Count}
	}
	return &data.HistogramResult{
		ResolvedInterval: resolved,
		Buckets:          buckets,
	}, nil
}

// applyCriteria runs every criterion through its predicate binding,
// mirroring Search's filter semantics.
func (c *histogramTemplateImpl[E, I]) applyCriteria(inner *gorm.DB, criteria []*data.SearchPredicate) (*gorm.DB, error) {
	for _, p := range criteria {
		b, ok := c.bindingsMap[p.Name]
		if !ok {
			return nil, fmt.Errorf("%w: %q", ErrUnknownSearchPredicate, p.Name)
		}
		next, bindErr := b.Binder(inner, p)
		if bindErr != nil {
			return nil, bindErr
		}
		inner = next
	}
	return inner, nil
}

// postgresBucketSpec returns the SQL expression that maps a timestamp
// column to its bucket start, plus the interval string to feed
// generate_series and the column-shift width. Calendar units use
// date_trunc; sub-day fixed widths fall back to epoch-modulo math
// because date_trunc has no step parameter.
func postgresBucketSpec(ri data.HistogramInterval, column string) (truncExpr, intervalSQL string, err error) {
	switch ri.Unit {
	case data.HistogramIntervalMinute:
		if ri.Step <= 1 {
			return fmt.Sprintf("date_trunc('minute', %s)", column), "1 minute", nil
		}
		seconds := ri.Step * 60
		return fmt.Sprintf("to_timestamp(floor(extract(epoch from %s) / %d) * %d)", column, seconds, seconds),
			fmt.Sprintf("%d minutes", ri.Step), nil
	case data.HistogramIntervalHour:
		if ri.Step <= 1 {
			return fmt.Sprintf("date_trunc('hour', %s)", column), "1 hour", nil
		}
		seconds := ri.Step * 3600
		return fmt.Sprintf("to_timestamp(floor(extract(epoch from %s) / %d) * %d)", column, seconds, seconds),
			fmt.Sprintf("%d hours", ri.Step), nil
	case data.HistogramIntervalDay:
		return fmt.Sprintf("date_trunc('day', %s)", column), "1 day", nil
	case data.HistogramIntervalWeek:
		return fmt.Sprintf("date_trunc('week', %s)", column), "1 week", nil
	case data.HistogramIntervalMonth:
		return fmt.Sprintf("date_trunc('month', %s)", column), "1 month", nil
	default:
		// HistogramIntervalAuto must be resolved by the caller before
		// bucket-spec computation.
		return "", "", fmt.Errorf("unsupported histogram interval unit %d", ri.Unit)
	}
}

// postgresAlignExpr returns the bucket-aligned expression for a bound
// timestamp parameter (typically the lower bound passed to
// generate_series). The shape mirrors postgresBucketSpec but accepts the
// bound as a placeholder so callers bind it as a query arg.
func postgresAlignExpr(ri data.HistogramInterval, placeholder string) string {
	switch ri.Unit {
	case data.HistogramIntervalMinute:
		if ri.Step <= 1 {
			return fmt.Sprintf("date_trunc('minute', %s)", placeholder)
		}
		seconds := ri.Step * 60
		return fmt.Sprintf("to_timestamp(floor(extract(epoch from %s) / %d) * %d)", placeholder, seconds, seconds)
	case data.HistogramIntervalHour:
		if ri.Step <= 1 {
			return fmt.Sprintf("date_trunc('hour', %s)", placeholder)
		}
		seconds := ri.Step * 3600
		return fmt.Sprintf("to_timestamp(floor(extract(epoch from %s) / %d) * %d)", placeholder, seconds, seconds)
	case data.HistogramIntervalDay:
		return fmt.Sprintf("date_trunc('day', %s)", placeholder)
	case data.HistogramIntervalWeek:
		return fmt.Sprintf("date_trunc('week', %s)", placeholder)
	case data.HistogramIntervalMonth:
		return fmt.Sprintf("date_trunc('month', %s)", placeholder)
	default:
		return placeholder
	}
}
