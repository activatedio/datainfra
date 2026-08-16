package gorm_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/activatedio/datainfra/pkg/data"
	dgorm "github.com/activatedio/datainfra/pkg/data/gorm"
)

// TestPostgresBucketSpec_AllUnits covers every supported
// HistogramIntervalUnit, locking in the SQL fragment shape the
// histogram template will splice into the generated query. Calendar
// units (Day/Week/Month) use date_trunc; sub-day fixed widths fall
// back to epoch-modulo because date_trunc has no step parameter.
//
// The function is unexported, so we exercise it via the exported
// runtime wrapper TestableBucketSpec defined alongside it.
func TestPostgresBucketSpec_AllUnits(t *testing.T) {
	cases := []struct {
		name            string
		interval        data.HistogramInterval
		wantTruncExpr   string
		wantIntervalSQL string
	}{
		{
			name:            "minute step 1",
			interval:        data.HistogramInterval{Unit: data.HistogramIntervalMinute, Step: 1},
			wantTruncExpr:   "date_trunc('minute', created_at)",
			wantIntervalSQL: "1 minute",
		},
		{
			name:            "minute step 0 falls through to 1",
			interval:        data.HistogramInterval{Unit: data.HistogramIntervalMinute, Step: 0},
			wantTruncExpr:   "date_trunc('minute', created_at)",
			wantIntervalSQL: "1 minute",
		},
		{
			name:            "minute step 5 uses epoch modulo",
			interval:        data.HistogramInterval{Unit: data.HistogramIntervalMinute, Step: 5},
			wantTruncExpr:   "to_timestamp(floor(extract(epoch from created_at) / 300) * 300)",
			wantIntervalSQL: "5 minutes",
		},
		{
			name:            "hour step 1",
			interval:        data.HistogramInterval{Unit: data.HistogramIntervalHour, Step: 1},
			wantTruncExpr:   "date_trunc('hour', created_at)",
			wantIntervalSQL: "1 hour",
		},
		{
			name:            "hour step 3 uses epoch modulo",
			interval:        data.HistogramInterval{Unit: data.HistogramIntervalHour, Step: 3},
			wantTruncExpr:   "to_timestamp(floor(extract(epoch from created_at) / 10800) * 10800)",
			wantIntervalSQL: "3 hours",
		},
		{
			name:            "day",
			interval:        data.HistogramInterval{Unit: data.HistogramIntervalDay},
			wantTruncExpr:   "date_trunc('day', created_at)",
			wantIntervalSQL: "1 day",
		},
		{
			name:            "week",
			interval:        data.HistogramInterval{Unit: data.HistogramIntervalWeek},
			wantTruncExpr:   "date_trunc('week', created_at)",
			wantIntervalSQL: "1 week",
		},
		{
			name:            "month",
			interval:        data.HistogramInterval{Unit: data.HistogramIntervalMonth},
			wantTruncExpr:   "date_trunc('month', created_at)",
			wantIntervalSQL: "1 month",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			truncExpr, intervalSQL, err := dgorm.TestablePostgresBucketSpec(tt.interval, "created_at")
			require.NoError(t, err)
			assert.Equal(t, tt.wantTruncExpr, truncExpr)
			assert.Equal(t, tt.wantIntervalSQL, intervalSQL)
		})
	}
}

func TestPostgresBucketSpec_UnsupportedUnit(t *testing.T) {
	_, _, err := dgorm.TestablePostgresBucketSpec(data.HistogramInterval{Unit: 99}, "created_at")
	require.Error(t, err)
}

// TestPostgresAlignExpr mirrors TestPostgresBucketSpec_AllUnits for the
// generate_series anchor expression. The two helpers share their unit
// logic but take a placeholder rather than a column name, so the
// emitted SQL substitutes "?" (or in the runtime template, the actual
// bound timestamp) in for the column.
func TestPostgresAlignExpr(t *testing.T) {
	cases := []struct {
		name     string
		interval data.HistogramInterval
		want     string
	}{
		{
			name:     "minute step 1",
			interval: data.HistogramInterval{Unit: data.HistogramIntervalMinute, Step: 1},
			want:     "date_trunc('minute', ?::timestamptz)",
		},
		{
			name:     "minute step 5",
			interval: data.HistogramInterval{Unit: data.HistogramIntervalMinute, Step: 5},
			want:     "to_timestamp(floor(extract(epoch from ?::timestamptz) / 300) * 300)",
		},
		{
			name:     "day",
			interval: data.HistogramInterval{Unit: data.HistogramIntervalDay},
			want:     "date_trunc('day', ?::timestamptz)",
		},
		{
			name:     "month",
			interval: data.HistogramInterval{Unit: data.HistogramIntervalMonth},
			want:     "date_trunc('month', ?::timestamptz)",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := dgorm.TestablePostgresAlignExpr(tt.interval, "?::timestamptz")
			assert.Equal(t, tt.want, got)
		})
	}
}
