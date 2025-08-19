// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	imetrics "github.com/coze-dev/coze-loop/backend/infra/metrics"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func NewExperimentMetric(meter imetrics.Meter) metrics.ExptMetric {
	if meter == nil {
		return nil
	}
	var err error

	if exptEvalMtr, err = meter.NewMetric(exptEvalMtrName, []imetrics.MetricType{imetrics.MetricTypeCounter, imetrics.MetricTypeTimer}, exptEvalMtrTags()); err != nil {
		panic(errorx.Wrapf(err, "new metric fail"))
	}

	if exptItemEvalMtr, err = meter.NewMetric(exptItemEvalMtrName, []imetrics.MetricType{imetrics.MetricTypeCounter, imetrics.MetricTypeTimer}, exptItemEvalMtrTags()); err != nil {
		panic(errorx.Wrapf(err, "new metric fail"))
	}

	if exptTurnEvalMtr, err = meter.NewMetric(exptTurnEvalMtrName, []imetrics.MetricType{imetrics.MetricTypeCounter, imetrics.MetricTypeTimer}, exptTurnEvalMtrTags()); err != nil {
		panic(errorx.Wrapf(err, "new metric fail"))
	}

	if getExptResultMtr, err = meter.NewMetric(getExptResultMtrName, []imetrics.MetricType{imetrics.MetricTypeCounter}, getExptResultMtrTags()); err != nil {
		panic(errorx.Wrapf(err, "new metric fail"))
	}

	if calculateExptAggrResultMtr, err = meter.NewMetric(calculateExptAggrResultMtrName, []imetrics.MetricType{imetrics.MetricTypeCounter, imetrics.MetricTypeTimer}, calculateExptAggrResultTags()); err != nil {
		panic(errorx.Wrapf(err, "new metric fail"))
	}

	if exptTurnResultFilterMtr, err = meter.NewMetric(exptTurnResultFilterMtrName, []imetrics.MetricType{imetrics.MetricTypeCounter, imetrics.MetricTypeTimer}, exptTurnResultFilterTags()); err != nil {
		panic(errorx.Wrapf(err, "new metric fail"))
	}

	return &ExperimentMetricImpl{
		exptEvalMtr:                exptEvalMtr,
		exptItemMtr:                exptItemEvalMtr,
		exptTurnMtr:                exptTurnEvalMtr,
		getExptResultMtr:           getExptResultMtr,
		calculateExptAggrResultMtr: calculateExptAggrResultMtr,
		exptTurnResultFilterMtr:    exptTurnResultFilterMtr,
	}
}

type ExperimentMetricImpl struct {
	exptEvalMtr                imetrics.Metric
	exptItemMtr                imetrics.Metric
	exptTurnMtr                imetrics.Metric
	getExptResultMtr           imetrics.Metric
	calculateExptAggrResultMtr imetrics.Metric
	exptTurnResultFilterMtr    imetrics.Metric
}

var exptEvalMtr, exptItemEvalMtr, exptTurnEvalMtr, getExptResultMtr, calculateExptAggrResultMtr, exptTurnResultFilterMtr imetrics.Metric

const (
	exptEvalMtrName                = "expt_eval"
	exptItemEvalMtrName            = "expt_item_eval"
	exptTurnEvalMtrName            = "expt_turn_eval"
	getExptResultMtrName           = "get_expt_result"
	calculateExptAggrResultMtrName = "calculate_expt_aggr_result"
	exptTurnResultFilterMtrName    = "expt_turn_result_filter"

	runSuffix    = "run"
	resultSuffix = "result"
	zombieSuffix = "zombie"

	targetSuffix    = ".target"
	evaluatorSuffix = ".evaluator"

	throughputSuffix = ".throughput"
	latencySuffix    = ".latency"
	checkSuffix      = ".check"
)

const (
	tagSpaceID            = "space_id"
	tagIsErr              = "is_err"
	tagRetry              = "retry"
	tagMode               = "mode"
	tagStatus             = "status"
	tagCode               = "code"
	tagStable             = "stable"
	tagExptType           = "expt_type"
	tagDiff               = "diff_exist"
	tagActualOutputDiff   = "actual_output_diff"
	tagEvaluatorScoreDiff = "evaluator_score_diff"
)

func exptEvalMtrTags() []string {
	return []string{
		tagSpaceID,
		tagRetry,
		tagMode,
		tagStatus,
		tagExptType,
	}
}

func exptItemEvalMtrTags() []string {
	return []string{
		tagSpaceID,
		tagIsErr,
		tagRetry,
		tagMode,
		tagStatus,
		tagCode,
		tagStable,
		tagExptType,
	}
}

func exptTurnEvalMtrTags() []string {
	return []string{
		tagSpaceID,
		tagIsErr,
		tagMode,
		tagStatus,
		tagCode,
		tagStable,
	}
}

func getExptResultMtrTags() []string {
	return []string{
		tagSpaceID,
		tagIsErr,
	}
}

func calculateExptAggrResultTags() []string {
	return []string{
		tagSpaceID,
		tagIsErr,
		tagMode,
	}
}

func exptTurnResultFilterTags() []string {
	return []string{
		tagSpaceID,
		tagDiff,
		tagActualOutputDiff,
		tagEvaluatorScoreDiff,
		tagIsErr,
	}
}
