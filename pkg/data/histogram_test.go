package data_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/activatedio/datainfra/pkg/data"
)

// TestResolveAutoInterval covers the ladder of bucket widths chosen for a
// given window. Each case picks a window size that falls just before a
// rung transition so the chosen unit is the smallest one that still
// yields ≤ targetBuckets buckets across the window.
func TestResolveAutoInterval(t *testing.T) {

	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name          string
		window        time.Duration
		targetBuckets int
		expected      data.HistogramInterval
	}{
		{
			// 30 minutes / 1m = 30 buckets ≤ 50 → 1m
			name:     "30 minutes - picks 1 minute",
			window:   30 * time.Minute,
			expected: data.HistogramInterval{Unit: data.HistogramIntervalMinute, Step: 1},
		},
		{
			// 4 hours / 1m = 240 buckets > 50; / 5m = 48 ≤ 50 → 5m
			name:     "4 hours - picks 5 minutes",
			window:   4 * time.Hour,
			expected: data.HistogramInterval{Unit: data.HistogramIntervalMinute, Step: 5},
		},
		{
			// 24h / 30m = 48 ≤ 50 → 30m
			name:     "24 hours - picks 30 minutes",
			window:   24 * time.Hour,
			expected: data.HistogramInterval{Unit: data.HistogramIntervalMinute, Step: 30},
		},
		{
			// 7 days / 3h = 56 > 50; / 12h = 14 ≤ 50 → 12h
			name:     "7 days - picks 12 hours",
			window:   7 * 24 * time.Hour,
			expected: data.HistogramInterval{Unit: data.HistogramIntervalHour, Step: 12},
		},
		{
			// 60 days / 1d = 60 > 50; / 1w = ~8 ≤ 50 → week
			name:     "60 days - picks week",
			window:   60 * 24 * time.Hour,
			expected: data.HistogramInterval{Unit: data.HistogramIntervalWeek, Step: 1},
		},
		{
			// 2 years far exceeds week target → month (top of ladder we use)
			name:     "2 years - picks month",
			window:   2 * 365 * 24 * time.Hour,
			expected: data.HistogramInterval{Unit: data.HistogramIntervalMonth, Step: 1},
		},
		{
			// targetBuckets override: with target=20 across 24h, 1h = 24 > 20; 3h = 8 ≤ 20 → 3h
			name:          "24 hours - target 20 - picks 3 hours",
			window:        24 * time.Hour,
			targetBuckets: 20,
			expected:      data.HistogramInterval{Unit: data.HistogramIntervalHour, Step: 3},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := data.ResolveAutoInterval(now, now.Add(tt.window), tt.targetBuckets)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestResolveAutoInterval_ZeroWindow(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	got := data.ResolveAutoInterval(now, now, 0)
	assert.Equal(t, data.HistogramInterval{Unit: data.HistogramIntervalMinute, Step: 1}, got)
}

func TestResolveAutoInterval_ReversedBounds(t *testing.T) {
	// min > max should be treated the same as the absolute duration —
	// the resolver should not panic and should produce a sensible unit.
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	expected := data.ResolveAutoInterval(now, now.Add(4*time.Hour), 0)
	got := data.ResolveAutoInterval(now.Add(4*time.Hour), now, 0)
	assert.Equal(t, expected, got)
}
