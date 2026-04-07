package consumer

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
)

func TestSpanStats_InjectAndGet(t *testing.T) {
	ctx := NewSpanStatsContext(context.Background())

	tds := Traces{
		Tenant: "tenant_a",
		TraceData: []*entity.TraceData{
			{
				SpanList: loop_span.SpanList{
					{PSM: "svc-a"},
					{PSM: "svc-a"},
					{PSM: "svc-b"},
				},
			},
		},
	}
	InjectSpanCounts(ctx, tds)

	entryA := GetSpanStatsEntry(ctx, "tenant_a", "svc-a")
	assert.NotNil(t, entryA)
	assert.Equal(t, 2, entryA.InCount)
	assert.Equal(t, 0, entryA.TotalFiltered())
	assert.Equal(t, 0, entryA.TotalOutCount())

	entryB := GetSpanStatsEntry(ctx, "tenant_a", "svc-b")
	assert.NotNil(t, entryB)
	assert.Equal(t, 1, entryB.InCount)

	entries := GetSpanStatsEntries(ctx)
	assert.Len(t, entries, 2)
}

func TestSpanStats_AddFilteredSpans_ByNode(t *testing.T) {
	ctx := NewSpanStatsContext(context.Background())

	tds := Traces{
		Tenant: "tenant_a",
		TraceData: []*entity.TraceData{
			{
				SpanList: loop_span.SpanList{
					{PSM: "svc-a"},
					{PSM: "svc-a"},
					{PSM: "svc-a"},
				},
			},
		},
	}
	InjectSpanCounts(ctx, tds)

	AddFilteredSpans(ctx, "tenant_a", "svc-a", "exporter/ck_online", 1)
	AddFilteredSpans(ctx, "tenant_a", "svc-a", "exporter/ck_offline", 2)

	entry := GetSpanStatsEntry(ctx, "tenant_a", "svc-a")
	assert.NotNil(t, entry)
	assert.Equal(t, 3, entry.InCount)
	assert.Equal(t, 1, entry.GetFiltered("exporter/ck_online"))
	assert.Equal(t, 2, entry.GetFiltered("exporter/ck_offline"))
	assert.Equal(t, 3, entry.TotalFiltered())
}

func TestSpanStats_AddFilteredSpans_SameNodeAccumulates(t *testing.T) {
	ctx := NewSpanStatsContext(context.Background())

	AddFilteredSpans(ctx, "tenant_a", "svc-a", "processor/filter", 3)
	AddFilteredSpans(ctx, "tenant_a", "svc-a", "processor/filter", 2)

	entry := GetSpanStatsEntry(ctx, "tenant_a", "svc-a")
	assert.NotNil(t, entry)
	assert.Equal(t, 5, entry.GetFiltered("processor/filter"))
}

func TestSpanStats_AddFilteredSpans_NewEntry(t *testing.T) {
	ctx := NewSpanStatsContext(context.Background())

	AddFilteredSpans(ctx, "tenant_x", "svc-x", "node_a", 5)

	entry := GetSpanStatsEntry(ctx, "tenant_x", "svc-x")
	assert.NotNil(t, entry)
	assert.Equal(t, 0, entry.InCount)
	assert.Equal(t, 5, entry.GetFiltered("node_a"))
	assert.Equal(t, 5, entry.TotalFiltered())
}

func TestSpanStats_NilContext(t *testing.T) {
	ctx := context.Background()

	InjectSpanCounts(ctx, Traces{})
	AddFilteredSpans(ctx, "t", "p", "n", 1)
	AddOutCountSpans(ctx, "t", "p", "n", 1)

	assert.Nil(t, GetSpanStatsEntries(ctx))
	assert.Nil(t, GetSpanStatsEntry(ctx, "t", "p"))
}

func TestSpanStats_AddOutCountSpans_ByPipeline(t *testing.T) {
	ctx := NewSpanStatsContext(context.Background())

	tds := Traces{
		Tenant: "tenant_a",
		TraceData: []*entity.TraceData{
			{
				SpanList: loop_span.SpanList{
					{PSM: "svc-a"},
					{PSM: "svc-a"},
					{PSM: "svc-a"},
				},
			},
		},
	}
	InjectSpanCounts(ctx, tds)

	AddOutCountSpans(ctx, "tenant_a", "svc-a", "exporter/ck_online", 2)
	AddOutCountSpans(ctx, "tenant_a", "svc-a", "exporter/ck_offline", 1)

	entry := GetSpanStatsEntry(ctx, "tenant_a", "svc-a")
	assert.NotNil(t, entry)
	assert.Equal(t, 3, entry.InCount)
	assert.Equal(t, 2, entry.GetOutCount("exporter/ck_online"))
	assert.Equal(t, 1, entry.GetOutCount("exporter/ck_offline"))
	assert.Equal(t, 3, entry.TotalOutCount())
}

func TestSpanStats_AddOutCountSpans_SamePipelineAccumulates(t *testing.T) {
	ctx := NewSpanStatsContext(context.Background())

	AddOutCountSpans(ctx, "tenant_a", "svc-a", "exporter/ck", 3)
	AddOutCountSpans(ctx, "tenant_a", "svc-a", "exporter/ck", 2)

	entry := GetSpanStatsEntry(ctx, "tenant_a", "svc-a")
	assert.NotNil(t, entry)
	assert.Equal(t, 5, entry.GetOutCount("exporter/ck"))
}

func TestSpanStats_AddOutCountSpans_NewEntry(t *testing.T) {
	ctx := NewSpanStatsContext(context.Background())

	AddOutCountSpans(ctx, "tenant_x", "svc-x", "pipeline_a", 7)

	entry := GetSpanStatsEntry(ctx, "tenant_x", "svc-x")
	assert.NotNil(t, entry)
	assert.Equal(t, 0, entry.InCount)
	assert.Equal(t, 0, entry.TotalFiltered())
	assert.Equal(t, 7, entry.GetOutCount("pipeline_a"))
	assert.Equal(t, 7, entry.TotalOutCount())
}

func TestInjectConsumer_InjectsStats(t *testing.T) {
	var capturedCtx context.Context
	inner := &ctxCapturingConsumer{capture: &capturedCtx}

	ic := NewInjectConsumer(inner)

	tds := Traces{
		Tenant: "tenant_a",
		TraceData: []*entity.TraceData{
			{
				SpanList: loop_span.SpanList{
					{PSM: "svc-a"},
					{PSM: "svc-b"},
				},
			},
		},
	}

	err := ic.ConsumeTraces(context.Background(), tds)
	assert.NoError(t, err)

	entries := GetSpanStatsEntries(capturedCtx)
	assert.Len(t, entries, 2)

	entryA := GetSpanStatsEntry(capturedCtx, "tenant_a", "svc-a")
	assert.NotNil(t, entryA)
	assert.Equal(t, 1, entryA.InCount)

	entryB := GetSpanStatsEntry(capturedCtx, "tenant_a", "svc-b")
	assert.NotNil(t, entryB)
	assert.Equal(t, 1, entryB.InCount)
}

type ctxCapturingConsumer struct {
	capture *context.Context
}

func (c *ctxCapturingConsumer) ConsumeTraces(ctx context.Context, tds Traces) error {
	*c.capture = ctx
	return nil
}
