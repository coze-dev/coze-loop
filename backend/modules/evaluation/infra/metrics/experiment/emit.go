// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"strconv"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/metrics"
)

func (e ExperimentMetricImpl) EmitExptExecRun(spaceID, mode int64) {
	e.exptEvalMtr.Emit([]metrics.T{
		{Name: tagSpaceID, Value: strconv.FormatInt(spaceID, 10)},
		{Name: tagMode, Value: strconv.FormatInt(mode, 10)},
	}, metrics.Counter(1, metrics.WithSuffix(runSuffix+throughputSuffix)))
}

func (e ExperimentMetricImpl) EmitExptExecResult(spaceID, typ, status int64, start time.Time) {
	e.exptEvalMtr.Emit([]metrics.T{
		{Name: tagSpaceID, Value: strconv.FormatInt(spaceID, 10)},
		{Name: tagStatus, Value: strconv.FormatInt(status, 10)},
		{Name: tagExptType, Value: strconv.FormatInt(typ, 10)},
	}, metrics.Counter(1, metrics.WithSuffix(resultSuffix+throughputSuffix)),
		metrics.Timer(int64(time.Since(start).Seconds()), metrics.WithSuffix(resultSuffix+latencySuffix)))
}

func (e ExperimentMetricImpl) EmitItemExecEval(spaceID, mode int64, cnt int) {
	e.exptItemMtr.Emit([]metrics.T{
		{Name: tagSpaceID, Value: strconv.FormatInt(spaceID, 10)},
		{Name: tagMode, Value: strconv.FormatInt(mode, 10)},
	}, metrics.Counter(int64(cnt), metrics.WithSuffix(runSuffix+throughputSuffix)))
}

func (e ExperimentMetricImpl) EmitItemExecResult(spaceID, mode int64, isErr, retry, stable bool, code, startTime int64) {
	e.exptItemMtr.Emit([]metrics.T{
		{Name: tagSpaceID, Value: strconv.FormatInt(spaceID, 10)},
		{Name: tagMode, Value: strconv.FormatInt(mode, 10)},
		{Name: tagIsErr, Value: strconv.FormatBool(isErr)},
		{Name: tagRetry, Value: strconv.FormatBool(retry)},
		{Name: tagCode, Value: strconv.FormatInt(code, 10)},
		{Name: tagStable, Value: strconv.FormatBool(stable)},
	}, metrics.Counter(1, metrics.WithSuffix(resultSuffix+throughputSuffix)),
		metrics.Timer(time.Now().Unix()-startTime, metrics.WithSuffix(resultSuffix+latencySuffix)))
}

func (e ExperimentMetricImpl) EmitZombies(spaceID, mode, exptTyp, cnt int64) {
	e.exptItemMtr.Emit([]metrics.T{
		{Name: tagSpaceID, Value: strconv.FormatInt(spaceID, 10)},
		{Name: tagMode, Value: strconv.FormatInt(mode, 10)},
		{Name: tagExptType, Value: strconv.FormatInt(exptTyp, 10)},
	}, metrics.Counter(cnt, metrics.WithSuffix(zombieSuffix+throughputSuffix)))
}

func (e ExperimentMetricImpl) EmitTurnExecEval(spaceID, mode int64) {
	e.exptTurnMtr.Emit([]metrics.T{
		{Name: tagSpaceID, Value: strconv.FormatInt(spaceID, 10)},
		{Name: tagMode, Value: strconv.FormatInt(mode, 10)},
	}, metrics.Counter(1, metrics.WithSuffix(runSuffix+throughputSuffix)))
}

func (e ExperimentMetricImpl) EmitTurnExecResult(spaceID, mode int64, isErr, stable bool, code int64, startTime time.Time) {
	e.exptTurnMtr.Emit([]metrics.T{
		{Name: tagSpaceID, Value: strconv.FormatInt(spaceID, 10)},
		{Name: tagMode, Value: strconv.FormatInt(mode, 10)},
		{Name: tagIsErr, Value: strconv.FormatBool(isErr)},
		{Name: tagCode, Value: strconv.FormatInt(code, 10)},
		{Name: tagStable, Value: strconv.FormatBool(stable)},
	}, metrics.Counter(1, metrics.WithSuffix(resultSuffix+throughputSuffix)),
		metrics.Timer(int64(time.Since(startTime).Seconds()), metrics.WithSuffix(resultSuffix+latencySuffix)))
}

func (e ExperimentMetricImpl) EmitTurnExecTargetResult(spaceID int64, isErr bool) {
	e.exptTurnMtr.Emit([]metrics.T{
		{Name: tagSpaceID, Value: strconv.FormatInt(spaceID, 10)},
		{Name: tagIsErr, Value: strconv.FormatBool(isErr)},
	}, metrics.Counter(1, metrics.WithSuffix(resultSuffix+targetSuffix+throughputSuffix)))
}

func (e ExperimentMetricImpl) EmitTurnExecEvaluatorResult(spaceID int64, isErr bool) {
	e.exptTurnMtr.Emit([]metrics.T{
		{Name: tagSpaceID, Value: strconv.FormatInt(spaceID, 10)},
		{Name: tagIsErr, Value: strconv.FormatBool(isErr)},
	}, metrics.Counter(1, metrics.WithSuffix(resultSuffix+evaluatorSuffix+throughputSuffix)))
}

func (e ExperimentMetricImpl) EmitGetExptResult(spaceID int64, isErr bool) {
	e.getExptResultMtr.Emit([]metrics.T{
		{Name: tagSpaceID, Value: strconv.FormatInt(spaceID, 10)},
		{Name: tagIsErr, Value: strconv.FormatBool(isErr)},
	}, metrics.Counter(1, metrics.WithSuffix("throughput")))
}

func (e ExperimentMetricImpl) EmitCalculateExptAggrResult(spaceID, mode int64, isErr bool, startTime int64) {
	e.calculateExptAggrResultMtr.Emit([]metrics.T{
		{Name: tagSpaceID, Value: strconv.FormatInt(spaceID, 10)},
		{Name: tagIsErr, Value: strconv.FormatBool(isErr)},
		{Name: tagMode, Value: strconv.FormatInt(mode, 10)},
	}, metrics.Counter(1, metrics.WithSuffix("throughput")),
		metrics.Timer(time.Now().Unix()-startTime, metrics.WithSuffix(latencySuffix)))
}

func (e ExperimentMetricImpl) EmitExptTurnResultFilterCheck(spaceID int64, evaluatorScoreDiff, actualOutputDiff, diff, resultMissing bool) {
	e.exptTurnResultFilterMtr.Emit([]metrics.T{
		{Name: tagSpaceID, Value: strconv.FormatInt(spaceID, 10)},
		{Name: tagActualOutputDiff, Value: strconv.FormatBool(actualOutputDiff)},
		{Name: tagEvaluatorScoreDiff, Value: strconv.FormatBool(evaluatorScoreDiff)},
		{Name: tagDiff, Value: strconv.FormatBool(diff)},
		{Name: tagResultMissing, Value: strconv.FormatBool(resultMissing)},
	}, metrics.Counter(1, metrics.WithSuffix("check")))
}

// nolint:byted_s_args_length_limit
func (e ExperimentMetricImpl) EmitExptTurnResultFilterQueryLatency(spaceID, startTime int64, isErr bool) {
	e.exptTurnResultFilterMtr.Emit([]metrics.T{
		{Name: tagSpaceID, Value: strconv.FormatInt(spaceID, 10)},
		{Name: tagIsErr, Value: strconv.FormatBool(isErr)},
	}, metrics.Timer(time.Now().Unix()-startTime, metrics.WithSuffix("latency")))
}
