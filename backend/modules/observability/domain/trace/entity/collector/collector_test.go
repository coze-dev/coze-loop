// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	mqmocks "github.com/coze-dev/coze-loop/backend/infra/mq/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/collector/exporter"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/collector/processor"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/collector/receiver"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/collector/exporter/clickhouseexporter"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/collector/processor/queueprocessor"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/collector/receiver/rmqreceiver"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
	confmocks "github.com/coze-dev/coze-loop/backend/pkg/conf/mocks"
)

var collectorCfg = map[string]any{
	"receivers": map[string]any{
		"rmq/default": map[string]any{
			"addr":           []string{"a"},
			"consumer_group": "11",
			"topic":          "22",
		},
	},
	"processors": map[string]any{
		"queue/default": map[string]any{
			"pool_name":         "a",
			"max_pool_size":     100,
			"queue_size":        100,
			"max_batch_size":    100,
			"tick_intervals_ms": 1000,
			"shard_count":       4,
		},
	},
	"exporters": map[string]any{
		"clickhouse/default": nil,
	},
	"tenants": map[string]any{
		"cozeloop": map[string]any{
			"receivers":  []string{"rmq/default"},
			"processors": []string{"queue/default"},
			"exporters":  []string{"clickhouse/default"},
		},
	},
}

func TestCollector_Run(t *testing.T) {
	type fields struct {
		conf               conf.IConfigLoader
		receiverFactories  []receiver.Factory
		processorFactories []processor.Factory
		exporterFactories  []exporter.Factory
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		wantErr      bool
	}{
		{
			name: "run collector successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockConf := confmocks.NewMockIConfigLoader(ctrl)
				mockConf.EXPECT().Get(gomock.Any(), gomock.Any()).Return(collectorCfg)
				mqConsumer := mqmocks.NewMockIConsumer(ctrl)
				mqConsumer.EXPECT().Start().Return(nil)
				mqConsumer.EXPECT().Close().Return(nil)
				mqConsumer.EXPECT().RegisterHandler(gomock.Any()).Return()
				mqFactory := mqmocks.NewMockIFactory(ctrl)
				mqFactory.EXPECT().NewConsumer(gomock.Any()).Return(mqConsumer, nil)
				return fields{
					conf: mockConf,
					receiverFactories: []receiver.Factory{
						rmqreceiver.NewFactory(mqFactory),
					},
					processorFactories: []processor.Factory{
						queueprocessor.NewFactory(),
					},
					exporterFactories: []exporter.Factory{
						clickhouseexporter.NewFactory(nil),
					},
				}
			},
			wantErr: false,
		},
		{
			name: "run collector failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockConf := confmocks.NewMockIConfigLoader(ctrl)
				mockConf.EXPECT().Get(gomock.Any(), gomock.Any()).Return(collectorCfg)
				mqConsumer := mqmocks.NewMockIConsumer(ctrl)
				mqConsumer.EXPECT().Start().Return(fmt.Errorf("err"))
				mqConsumer.EXPECT().Close().Return(nil)
				mqConsumer.EXPECT().RegisterHandler(gomock.Any()).Return()
				mqFactory := mqmocks.NewMockIFactory(ctrl)
				mqFactory.EXPECT().NewConsumer(gomock.Any()).Return(mqConsumer, nil)
				return fields{
					conf: mockConf,
					receiverFactories: []receiver.Factory{
						rmqreceiver.NewFactory(mqFactory),
					},
					processorFactories: []processor.Factory{
						queueprocessor.NewFactory(),
					},
					exporterFactories: []exporter.Factory{
						clickhouseexporter.NewFactory(nil),
					},
				}
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			receiverMap, _ := receiver.MakeFactoryMap(fields.receiverFactories...)
			processorMap, _ := processor.MakeFactoryMap(fields.processorFactories...)
			exporterMap, _ := exporter.MakeFactoryMap(fields.exporterFactories...)
			c, err := New(Settings{
				Factories: func() (Factories, error) {
					return Factories{
						Receivers:  receiverMap,
						Processors: processorMap,
						Exporters:  exporterMap,
					}, nil
				},
				ConfigProvider: NewConfigProvider(fields.conf),
			})
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()
			err = c.RunInOne(ctx, func() error {
				return nil
			})
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}
