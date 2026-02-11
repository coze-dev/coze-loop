// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rocket

import (
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/samber/lo"

	"github.com/coze-dev/coze-loop/backend/infra/mq"
)

const (
	ExptScheduleEventRMQKey         = "expt_scheduler_event_rmq"
	ExptRecordEvalEventRMQKey       = "expt_record_eval_event_rmq"
	ExptAggrCalculateEventRMQKey    = "expt_aggr_calculate_event_rmq"
	ExptOnlineEvalResultRMQKey      = "expt_online_eval_result_rmq"
	EvaluatorRecordCorrectionRMQKey = "evaluator_record_correction_rmq"
	ExptTurnResultFilterRMQKey      = "expt_turn_result_filter_rmq"
	ExptExportCSVEventRMQKey        = "expt_export_csv_event_rmq"
	ExptAnalysisEventRMQKey         = "expt_analysis_event_rmq"
)

type RMQConf struct {
	Addr  string `json:"addr" mapstructure:"addr"`
	Topic string `json:"topic" mapstructure:"topic"`

	ProduceTimeout time.Duration `json:"produce_timeout" mapstructure:"produce_timeout"`
	RetryTimes     int           `json:"retry_times" mapstructure:"retry_times"`
	ProducerGroup  string        `json:"producer_group" mapstructure:"producer_group"`

	ConsumerGroup  string        `json:"consumer_group" mapstructure:"consumer_group"`
	WorkerNum      int           `json:"worker_num" mapstructure:"worker_num"`
	ConsumeTimeout time.Duration `json:"consume_timeout" mapstructure:"consume_timeout"`

	AccessKey    *string `json:"access_key" mapstructure:"access_key"`
	AccessSecret *string `json:"access_secret" mapstructure:"access_secret"`
}

func (c *RMQConf) Valid() bool {
	return len(c.Addr) > 0 && len(c.Topic) > 0 && len(c.ConsumerGroup) > 0
}

func (c *RMQConf) ToProducerCfg() mq.ProducerConfig {
	nameSrvAddrs := []string{c.Addr}
	return mq.ProducerConfig{
		Addr:           lo.Ternary(len(nameSrvAddrs) > 0, nameSrvAddrs, []string{c.Addr}),
		ProduceTimeout: c.ProduceTimeout,
		RetryTimes:     c.RetryTimes,
		ProducerGroup:  gptr.Of(c.ProducerGroup),
		AccessKey:      c.AccessKey,
		AccessSecret:   c.AccessSecret,
	}
}

func (c *RMQConf) ToConsumerCfg() mq.ConsumerConfig {
	nameSrvAddrs := []string{c.Addr}
	return mq.ConsumerConfig{
		Addr:                 lo.Ternary(len(nameSrvAddrs) > 0, nameSrvAddrs, []string{c.Addr}),
		Topic:                c.Topic,
		ConsumerGroup:        c.ConsumerGroup,
		ConsumeGoroutineNums: c.WorkerNum,
		ConsumeTimeout:       c.ConsumeTimeout,
		AccessKey:            c.AccessKey,
		AccessSecret:         c.AccessSecret,
	}
}
