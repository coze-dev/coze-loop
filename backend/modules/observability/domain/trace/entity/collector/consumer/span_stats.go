package consumer

import "context"

type spanStatsKey struct{}

type SpanStatsEntry struct {
	Tenant        string
	PSM           string
	InCount       int
	FilteredCount map[string]int
}

func (e *SpanStatsEntry) TotalFiltered() int {
	total := 0
	for _, c := range e.FilteredCount {
		total += c
	}
	return total
}

func (e *SpanStatsEntry) GetFiltered(node string) int {
	return e.FilteredCount[node]
}

type SpanStats struct {
	entries map[string]*SpanStatsEntry
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
	for _, trace := range tds.TraceData {
		for _, span := range trace.SpanList {
			key := statsKey(tds.Tenant, span.PSM)
			entry, ok := stats.entries[key]
			if !ok {
				entry = &SpanStatsEntry{
					Tenant:        tds.Tenant,
					PSM:           span.PSM,
					FilteredCount: make(map[string]int),
				}
				stats.entries[key] = entry
			}
			entry.InCount++
		}
	}
}

func AddFilteredSpans(ctx context.Context, tenant, psm, pipeline string, count int) {
	stats := getSpanStats(ctx)
	if stats == nil {
		return
	}
	key := statsKey(tenant, psm)
	entry, ok := stats.entries[key]
	if !ok {
		entry = &SpanStatsEntry{
			Tenant:        tenant,
			PSM:           psm,
			FilteredCount: make(map[string]int),
		}
		stats.entries[key] = entry
	}
	entry.FilteredCount[pipeline] += count
}

func GetSpanStatsEntries(ctx context.Context) []*SpanStatsEntry {
	stats := getSpanStats(ctx)
	if stats == nil {
		return nil
	}
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
	return stats.entries[statsKey(tenant, psm)]
}
