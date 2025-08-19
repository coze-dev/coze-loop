// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package queueprocessor

import (
	"context"
	"math/rand"
	"strconv"
	"sync"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/collector/component"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/collector/consumer"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/collector/processor"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
)

type nextConsumerMock struct {
	lock   sync.Mutex
	traces []consumer.Traces
	count  int
}

func (c *nextConsumerMock) ConsumeTraces(ctx context.Context, td consumer.Traces) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.traces = append(c.traces, td)
	c.count += td.SpansCount()
	return nil
}

func TestQueueProcessor(t *testing.T) {
	for round := 1; round <= 50; round++ {
		t.Logf("========== Round %d", round)
		nextConsumer := &nextConsumerMock{}
		qProcessor, err := createTracesProcessor(context.Background(), processor.CreateSettings{
			ID: component.ID{},
		}, &Config{
			PoolName:        "default",
			MaxPoolSize:     1000,
			QueueSize:       1000,
			MaxBatchSize:    100,
			TickIntervalsMs: 1000,
			ShardCount:      5,
		}, nextConsumer)
		if err != nil {
			t.Fatal(err)
		}
		done := make(chan struct{})
		if err := qProcessor.Start(context.Background()); err != nil {
			t.Fatal(err)
		}
		expectedCount := 0
		go func() {
			for i := 0; i < 1000; i++ {
				for j := 0; j < 10; j++ {
					tmp := strconv.Itoa(i % 3)
					spanList := make(loop_span.SpanList, rand.Int()&4)
					expectedCount += len(spanList)
					for i := range spanList {
						spanList[i] = new(loop_span.Span)
						spanList[i].SpanID = tmp
					}
					if err := qProcessor.ConsumeTraces(context.Background(), consumer.Traces{
						Tenant: "default",
						TraceData: []*entity.TraceData{
							{
								Tenant: "default",
								TenantInfo: entity.TenantInfo{
									TTL: loop_span.TTL(tmp),
								},
								SpanList: spanList,
							},
						},
					}); err != nil {
						panic(err)
					}
				}
			}
			done <- struct{}{}
		}()
		<-done
		if err := qProcessor.Shutdown(context.Background()); err != nil {
			panic(err)
		}
		// check
		p := qProcessor.(*queueProcessor)
		for _, b := range p.batch.shards {
			if b.count() != 0 {
				panic("should left no spans")
			}
		}
		outputCount := 0
		for _, td := range nextConsumer.traces {
			outputCount += td.SpansCount()
		}
		t.Log(expectedCount, outputCount, nextConsumer.count)
		if expectedCount != outputCount || outputCount != nextConsumer.count {
			panic("traces count not consistent")
		}
	}
}
