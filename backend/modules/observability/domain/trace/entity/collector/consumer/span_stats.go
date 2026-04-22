package consumer

import (
	"context"
	"sync"
)

type spanStatsKey struct{}

type SpanStatsEntry struct {
	Tenant        string
	PSM           string
	InCount       int
	FilteredCount map[string]int
	OutCount      map[string]int
	spanStatsLock sync.Mutex
}

func (e *SpanStatsEntry) TotalFiltered() int {
	e.spanStatsLock.Lock()
	defer e.spanStatsLock.Unlock()
	total := 0
	for _, c := range e.FilteredCount {
		total += c
	}
	return total
}

func (e *SpanStatsEntry) GetFiltered(node string) int {
	e.spanStatsLock.Lock()
	defer e.spanStatsLock.Unlock()
	return e.FilteredCount[node]
}

func (e *SpanStatsEntry) TotalOutCount() int {
	e.spanStatsLock.Lock()
	defer e.spanStatsLock.Unlock()
	total := 0
	for _, c := range e.OutCount {
		total += c
	}
	return total
}

func (e *SpanStatsEntry) GetOutCount(pipeline string) int {
	e.spanStatsLock.Lock()
	defer e.spanStatsLock.Unlock()
	return e.OutCount[pipeline]
}

type SpanStats struct {
	entries map[string]*SpanStatsEntry
	lock    sync.Mutex
}

func newSpanStats() *SpanStats {
	return &SpanStats{
		entries: make(map[string]*SpanStatsEntry),
	}
}

func statsKey(tenant, psm string) string {
	return tenant + "|" + psm
}

func NewSpanStatsContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, spanStatsKey{}, newSpanStats())
}

func getSpanStats(ctx context.Context) *SpanStats {
	v, _ := ctx.Value(spanStatsKey{}).(*SpanStats)
	return v
}

func InjectSpanCounts(ctx context.Context, tds Traces) {
	stats := getSpanStats(ctx)
	if stats == nil {
		return
	}
	stats.lock.Lock()
	defer stats.lock.Unlock()
	for _, trace := range tds.TraceData {
		for _, span := range trace.SpanList {
			key := statsKey(tds.Tenant, span.PSM)
			entry, ok := stats.entries[key]
			if !ok {
				entry = &SpanStatsEntry{
					Tenant:        tds.Tenant,
					PSM:           span.PSM,
					FilteredCount: make(map[string]int),
					OutCount:      make(map[string]int),
				}
				stats.entries[key] = entry
			}
			entry.spanStatsLock.Lock()
			entry.InCount++
			entry.spanStatsLock.Unlock()
		}
	}
}

// pipeline 整体再看下名字
func AddFilteredSpans(ctx context.Context, tenant, psm, pipeline string, count int) {
	stats := getSpanStats(ctx)
	if stats == nil {
		return
	}
	key := statsKey(tenant, psm)
	stats.lock.Lock()
	entry, ok := stats.entries[key]
	if !ok {
		entry = &SpanStatsEntry{
			Tenant:        tenant,
			PSM:           psm,
			FilteredCount: make(map[string]int),
			OutCount:      make(map[string]int),
		}
		stats.entries[key] = entry
	}
	stats.lock.Unlock()
	entry.spanStatsLock.Lock()
	entry.FilteredCount[pipeline] += count
	entry.spanStatsLock.Unlock()
}

func AddOutCountSpans(ctx context.Context, tenant, psm, pipeline string, count int) {
	stats := getSpanStats(ctx)
	if stats == nil {
		return
	}
	key := statsKey(tenant, psm)
	stats.lock.Lock()
	entry, ok := stats.entries[key]
	if !ok {
		entry = &SpanStatsEntry{
			Tenant:        tenant,
			PSM:           psm,
			FilteredCount: make(map[string]int),
			OutCount:      make(map[string]int),
		}
		stats.entries[key] = entry
	}
	stats.lock.Unlock()
	entry.spanStatsLock.Lock()
	entry.OutCount[pipeline] += count
	entry.spanStatsLock.Unlock()
}

func GetSpanStatsEntries(ctx context.Context) []*SpanStatsEntry {
	stats := getSpanStats(ctx)
	if stats == nil {
		return nil
	}
	stats.lock.Lock()
	defer stats.lock.Unlock()
	result := make([]*SpanStatsEntry, 0, len(stats.entries))
	for _, entry := range stats.entries {
		result = append(result, entry)
	}
	return result
}

func GetSpanStatsEntry(ctx context.Context, tenant, psm string) *SpanStatsEntry {
	stats := getSpanStats(ctx)
	if stats == nil {
		return nil
	}
	stats.lock.Lock()
	defer stats.lock.Unlock()
	return stats.entries[statsKey(tenant, psm)]
}
