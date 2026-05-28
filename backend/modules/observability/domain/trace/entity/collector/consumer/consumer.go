// Original Files: open-telemetry/opentelemetry-collector
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
// This file may have been modified by ByteDance Ltd.

package consumer

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
)

// 权益相关, 每一批上报的数据的元信息不同, 不能简单合并
type Traces struct {
	Tenant    string
	TraceData []*entity.TraceData
}

func (t *Traces) SpansCount() int {
	ret := 0
	for _, trace := range t.TraceData {
		ret += len(trace.SpanList)
	}
	return ret
}

func (t *Traces) SpansCountByPSM() map[string]int {
	result := make(map[string]int)
	for _, trace := range t.TraceData {
		for _, span := range trace.SpanList {
			result[span.PSM]++
		}
	}
	return result
}

type BaseConsumer interface{}

//go:generate mockgen -destination=mocks/consumer.go -package=mocks . Consumer
type Consumer interface {
	BaseConsumer
	ConsumeTraces(ctx context.Context, tds Traces) error
}

type fanoutConsumer struct {
	traces []Consumer
}

func (tsc *fanoutConsumer) ConsumeTraces(ctx context.Context, td Traces) error {
	g := errgroup.Group{}
	for i := 0; i < len(tsc.traces); i++ {
		component := tsc.traces[i]
		g.Go(func() error {
			defer func() {
				if r := recover(); r != nil {
					fmt.Println("Recovered in fanout", r)
				}
			}()
			err := component.ConsumeTraces(ctx, td)
			if err != nil {
				return err
			}
			return nil
		})
	}
	err := g.Wait()
	if err != nil {
		return err
	}
	return nil
}

func NewFanoutConsumer(tcs []Consumer) Consumer {
	tc := &fanoutConsumer{
		traces: make([]Consumer, 0),
	}
	tc.traces = append(tc.traces, tcs...)
	return tc
}
