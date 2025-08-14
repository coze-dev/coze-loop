// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import "time"

//go:generate mockgen -destination=mocks/expt.go -package=mocks . ExptMetric
type ExptMetric interface {
	ExptItemExecMetrics
	ExptTurnExecMtr
	ExptExecMetrics
	ExptResultMetrics
	ExptAggrResultMetrics
}

type ExptItemExecMetrics interface {
	EmitItemExecEval(spaceID, mode int64, cnt int)
	EmitItemExecResult(spaceID, mode int64, isErr, retry, stable bool, code, startTime int64)
	EmitZombies(spaceID, mode, exptTyp, cnt int64)
}

type ExptTurnExecMtr interface {
	EmitTurnExecEval(spaceID, mode int64)
	EmitTurnExecResult(spaceID, mode int64, isErr, stable bool, code int64, startTime time.Time)
	EmitTurnExecTargetResult(spaceID int64, isErr bool)
	EmitTurnExecEvaluatorResult(spaceID int64, isErr bool)
}

type ExptExecMetrics interface {
	EmitExptExecRun(spaceID, mode int64)
	EmitExptExecResult(spaceID, typ, status int64, start time.Time)
}

type ExptResultMetrics interface {
	EmitGetExptResult(spaceID int64, isErr bool)
	EmitExptTurnResultFilterCheck(spaceID int64, evaluatorScore, actualOutputDiff, diff bool)
	EmitExptTurnResultFilterQueryLatency(spaceID, startTime int64, isErr bool)
}

type ExptAggrResultMetrics interface {
	EmitCalculateExptAggrResult(spaceID, mode int64, isErr bool, startTime int64)
}
