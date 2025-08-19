// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"context"
	"strings"
	"time"

	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type ExptRunMode int32

const (
	// EvaluationModeSubmit 创建后提交
	EvaluationModeSubmit ExptRunMode = 1

	// EvaluationModeFailRetry 失败后全部重试
	EvaluationModeFailRetry ExptRunMode = 2

	// EvaluationModeAppend 追加模式
	EvaluationModeAppend ExptRunMode = 3
)

type ItemRunState int64

const (
	ItemRunState_Unknown ItemRunState = -1
	// Queuing
	ItemRunState_Queueing ItemRunState = 0
	// Processing
	ItemRunState_Processing ItemRunState = 1
	// Success
	ItemRunState_Success ItemRunState = 2
	// Failure
	ItemRunState_Fail ItemRunState = 3
	// Terminated
	ItemRunState_Terminal ItemRunState = 5
)

type TurnRunState int64

const (
	// Not started
	TurnRunState_Queueing TurnRunState = 0
	// Execution succeeded
	TurnRunState_Success TurnRunState = 1
	// Execution failed
	TurnRunState_Fail TurnRunState = 2
	// In progress
	TurnRunState_Processing TurnRunState = 3
	// Terminated
	TurnRunState_Terminal TurnRunState = 4
)

func IsTurnRunFinished(state TurnRunState) bool {
	return state == TurnRunState_Success || state == TurnRunState_Fail || state == TurnRunState_Terminal
}

func IsExptFinished(status ExptStatus) bool {
	return status == ExptStatus_Success || status == ExptStatus_Failed || status == ExptStatus_Terminated || status == ExptStatus_SystemTerminated
}

func IsItemRunFinished(state ItemRunState) bool {
	return state == ItemRunState_Fail || state == ItemRunState_Terminal || state == ItemRunState_Success
}

type ExptItemResultState int

const (
	ExptItemResultStateDefault  ExptItemResultState = 0
	ExptItemResultStateLogged   ExptItemResultState = 2
	ExptItemResultStateResulted ExptItemResultState = 1
)

type CreditCost int

const (
	CreditCostDefault CreditCost = 0
	CreditCostFree    CreditCost = 1
)

const (
	defaultDaemonInterval       = 20 * time.Second
	defaultZombieIntervalSecond = 60 * 60 * 24
	defaultItemEvalConcurNum    = 3
	defaultItemEvalInterval     = 20 * time.Second
	defaultSpaceExptConcurLimit = 200
	defaultItemZombieSecond     = 60 * 20
)

type ExptConsumerConf struct {
	ExptExecWorkerNum     int `json:"expt_exec_worker_num" mapstructure:"expt_exec_worker_num"`
	ExptItemEvalWorkerNum int `json:"expt_item_eval_worker_num" mapstructure:"expt_item_eval_worker_num"`

	ExptExecConf      *ExptExecConf           `json:"expt_exec_conf" mapstructure:"expt_exec_conf"`
	SpaceExptExecConf map[int64]*ExptExecConf `json:"space_expt_exec_conf" mapstructure:"space_expt_exec_conf"`
}

func (e *ExptConsumerConf) GetExptExecConf(spaceID int64) *ExptExecConf {
	if e == nil {
		return nil
	}
	if e.SpaceExptExecConf[spaceID] != nil {
		return e.SpaceExptExecConf[spaceID]
	}
	return e.ExptExecConf
}

type ExptExecConf struct {
	DaemonIntervalSecond int `json:"daemon_interval_second" mapstructure:"daemon_interval_second"`
	ZombieIntervalSecond int `json:"expt_zombie_second" mapstructure:"expt_zombie_second"`
	SpaceExptConcurLimit int `json:"space_expt_concur_limit" mapstructure:"space_expt_concur_limit"`

	ExptItemEvalConf *ExptItemEvalConf `json:"expt_item_eval_conf" mapstructure:"expt_item_eval_conf"`
}

func (e *ExptExecConf) GetSpaceExptConcurLimit() int {
	if e != nil && e.SpaceExptConcurLimit > 0 {
		return e.SpaceExptConcurLimit
	}
	return defaultSpaceExptConcurLimit
}

func (e *ExptExecConf) GetDaemonInterval() time.Duration {
	if e != nil && e.DaemonIntervalSecond > 0 {
		return time.Duration(e.DaemonIntervalSecond) * time.Second
	}
	return defaultDaemonInterval
}

func (e *ExptExecConf) GetZombieIntervalSecond() int {
	if e != nil && e.ZombieIntervalSecond > 0 {
		return e.ZombieIntervalSecond
	}
	return defaultZombieIntervalSecond
}

func (e *ExptExecConf) GetExptItemEvalConf() *ExptItemEvalConf {
	if e != nil {
		return e.ExptItemEvalConf
	}
	return nil
}

type ExptItemEvalConf struct {
	ConcurNum      int `json:"concur_num" mapstructure:"concur_num"`
	IntervalSecond int `json:"interval_second" mapstructure:"interval_second"`
	ZombieSecond   int `json:"zombie_second" mapstructure:"zombie_second"`
}

func (e *ExptItemEvalConf) GetConcurNum() int {
	if e != nil && e.ConcurNum > 0 {
		return e.ConcurNum
	}
	return defaultItemEvalConcurNum
}

func (e *ExptItemEvalConf) GetInterval() time.Duration {
	if e != nil && e.IntervalSecond > 0 {
		return time.Duration(e.IntervalSecond) * time.Second
	}
	return defaultItemEvalInterval
}

func (e *ExptItemEvalConf) GetZombieSecond() int {
	if e != nil && e.ZombieSecond > 0 {
		return e.ZombieSecond
	}
	return defaultItemZombieSecond
}

func DefaultExptConsumerConf() *ExptConsumerConf {
	return &ExptConsumerConf{
		ExptExecWorkerNum:     50,
		ExptItemEvalWorkerNum: 200,
	}
}

func DefaultExptErrCtrl() *ExptErrCtrl {
	return &ExptErrCtrl{}
}

type ExptErrCtrl struct {
	ErrRetryCtrl      *ErrRetryCtrl           `json:"err_retry_ctrl" mapstructure:"err_retry_ctrl"`
	SpaceErrRetryCtrl map[int64]*ErrRetryCtrl `json:"space_err_retry_ctrl" mapstructure:"space_err_retry_ctrl"`
	ResultErrConverts []*ResultErrConvert     `json:"result_err_converts" mapstructure:"result_err_converts"`
}

type ResultErrConvert struct {
	MatchedText string `json:"matched_text" mapstructure:"matched_text"`
	ToErrCode   int32  `json:"to_err_code" mapstructure:"to_err_code"`
	ToErrMsg    string `json:"to_err_msg" mapstructure:"to_err_msg"`
	AsDefault   bool   `json:"as_default" mapstructure:"as_default"`
}

func (r *ResultErrConvert) ConvertErrMsg(msg string) (bool, string) {
	if r == nil || len(msg) == 0 {
		return false, ""
	}
	if r.ToErrCode <= 0 && len(r.ToErrMsg) == 0 {
		return false, ""
	}
	if !r.AsDefault && (len(r.MatchedText) == 0 || !strings.Contains(msg, r.MatchedText)) {
		return false, ""
	}
	if r.ToErrCode > 0 {
		return true, errorx.ErrorWithoutStack(errorx.NewByCode(r.ToErrCode))
	}
	if len(r.ToErrMsg) > 0 {
		return true, r.ToErrMsg
	}
	return false, msg
}

func (e *ExptErrCtrl) GetErrRetryCtrl(spaceID int64) *ErrRetryCtrl {
	if e == nil {
		return &ErrRetryCtrl{}
	}
	if e.SpaceErrRetryCtrl[spaceID] != nil {
		return e.SpaceErrRetryCtrl[spaceID]
	}
	return e.ErrRetryCtrl
}

func (e *ExptErrCtrl) ConvertErrMsg(msg string) string {
	if e == nil || len(msg) == 0 {
		return ""
	}

	defaultConf := &ResultErrConvert{}
	for _, conf := range e.ResultErrConverts {
		if conf.AsDefault {
			defaultConf = conf
			continue
		}
		if convert, cm := conf.ConvertErrMsg(msg); convert {
			return cm
		}
	}

	_, cm := defaultConf.ConvertErrMsg(msg)
	return cm
}

type ErrRetryCtrl struct {
	RetryConf    *RetryConf            `json:"retry_conf" mapstructure:"retry_conf"`
	ErrRetryConf map[string]*RetryConf `json:"err_retry_conf" mapstructure:"err_retry_conf"`
}

func (e *ErrRetryCtrl) GetRetryConf(err error) *RetryConf {
	if e == nil || err == nil {
		return nil
	}

	errMsg := err.Error()
	for str, conf := range e.ErrRetryConf {
		if strings.Contains(errMsg, str) {
			return conf
		}
	}

	return e.RetryConf
}

type RetryConf struct {
	RetryTimes          int  `json:"retry_times" mapstructure:"retry_times"`
	RetryIntervalSecond int  `json:"retry_interval_second" mapstructure:"retry_interval_second"`
	IsInDebt            bool `json:"is_in_debt" mapstructure:"is_in_debt"`
}

func (e *RetryConf) GetRetryTimes() int {
	if e != nil {
		return e.RetryTimes
	}
	return 0
}

func (e *RetryConf) GetRetryInterval() time.Duration {
	if e != nil && e.RetryIntervalSecond > 0 {
		return time.Duration(e.RetryIntervalSecond) * time.Second
	}
	return time.Second * 20
}

type QuotaSpaceExpt struct {
	ExptID2RunTime map[int64]int64 // id -> unix
}

func (q *QuotaSpaceExpt) Serialize() ([]byte, error) {
	bytes, err := json.Marshal(q)
	if err != nil {
		return nil, errorx.Wrapf(err, "QuotaSpaceExpt json marshal failed")
	}
	return bytes, nil
}

type ExptItemEvalCtx struct {
	Event *ExptItemEvalEvent

	Expt *Experiment

	EvalSetItem *EvaluationSetItem

	ExistItemEvalResult *ExptItemEvalResult
}

func (e *ExptItemEvalCtx) GetRecordEvalLogID(ctx context.Context) (logID string) {
	itemRunLog := e.GetExistItemResultLog()

	defer func() {
		logs.CtxInfo(ctx, "GetRecordEvalLogID with log_id: %v", logID)
	}()

	if itemRunLog == nil || len(itemRunLog.LogID) == 0 {
		return logs.NewLogID()
	}

	return itemRunLog.LogID
}

func (e *ExptItemEvalCtx) GetTurnEvalLogID(ctx context.Context, turnID int64) (logID string) {
	turnRunLog := e.GetExistTurnResultRunLog(turnID)

	defer func() {
		logs.CtxInfo(ctx, "GetTurnEvalLogID with log_id: %v", logID)
	}()

	if turnRunLog == nil || len(turnRunLog.LogID) == 0 {
		return logs.NewLogID()
	}

	return turnRunLog.LogID
}

func (e *ExptItemEvalCtx) GetExistTurnResultRunLog(turnID int64) *ExptTurnResultRunLog {
	return e.GetExistTurnResultLogs()[turnID]
}

func (e *ExptItemEvalCtx) GetExistItemResultLog() *ExptItemResultRunLog {
	if e == nil || e.ExistItemEvalResult == nil {
		return nil
	}
	return e.ExistItemEvalResult.ItemResultRunLog
}

func (e *ExptItemEvalCtx) GetExistTurnResultLogs() map[int64]*ExptTurnResultRunLog {
	if e == nil || e.ExistItemEvalResult == nil {
		return nil
	}
	return e.ExistItemEvalResult.TurnResultRunLogs
}

type ExptTurnEvalCtx struct {
	*ExptItemEvalCtx
	Turn              *Turn
	ExptTurnRunResult *ExptTurnRunResult
	History           []*Message
	Ext               map[string]string
}

type ExptTurnRunResult struct {
	TargetResult     *EvalTargetRecord
	EvaluatorResults map[int64]*EvaluatorRecord
	EvalErr          error
}

func (e *ExptTurnRunResult) SetTargetResult(er *EvalTargetRecord) *ExptTurnRunResult {
	e.TargetResult = er
	return e
}

func (e *ExptTurnRunResult) SetEvaluatorResults(er map[int64]*EvaluatorRecord) *ExptTurnRunResult {
	e.EvaluatorResults = er
	return e
}

func (e *ExptTurnRunResult) SetEvalErr(err error) *ExptTurnRunResult {
	e.EvalErr = err
	return e
}

func (e *ExptTurnRunResult) GetEvalErr() error {
	if e != nil {
		return e.EvalErr
	}
	return nil
}

func (e *ExptTurnRunResult) GetEvaluatorRecord(evaluatorVersionID int64) *EvaluatorRecord {
	if e == nil {
		return nil
	}
	return e.EvaluatorResults[evaluatorVersionID]
}

//go:generate  mockgen -destination  ./mocks/expt_scheduler_mock.go  --package mocks . ExptSchedulerMode
type ExptSchedulerMode interface {
	Mode() ExptRunMode
	ExptStart(ctx context.Context, event *ExptScheduleEvent, expt *Experiment) error
	ScanEvalItems(ctx context.Context, event *ExptScheduleEvent, expt *Experiment) (toSubmit, incomplete, complete []*ExptEvalItem, err error)
	ExptEnd(ctx context.Context, event *ExptScheduleEvent, expt *Experiment, toSubmit, incomplete int) (nextTick bool, err error)
	ScheduleStart(ctx context.Context, event *ExptScheduleEvent, expt *Experiment) error
	ScheduleEnd(ctx context.Context, event *ExptScheduleEvent, expt *Experiment, toSubmit, incomplete int) error
	NextTick(ctx context.Context, event *ExptScheduleEvent, nextTick bool) error
	PublishResult(ctx context.Context, turnEvaluatorRefs []*ExptTurnEvaluatorResultRef, event *ExptScheduleEvent) error
}

type CKDBConfig struct {
	ExptTurnResultFilterDBName string `json:"expt_turn_result_filter_db_name" mapstructure:"expt_turn_result_filter_db_name"`
	DatasetItemsSnapshotDBName string `json:"dataset_items_snapshot_db_name" mapstructure:"dataset_items_snapshot_db_name"`
}
