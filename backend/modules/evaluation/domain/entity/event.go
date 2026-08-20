// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/pkg/ctxcache"
)

type ExptScheduleEvent struct {
	SpaceID     int64
	ExptID      int64
	ExptRunID   int64
	ExptRunMode ExptRunMode
	ExptType    ExptType

	CreatedAt int64
	Ext       map[string]string
	Session   *Session

	ItemRetryTimes       int
	ExecEvalSetItemIDs   []int64
	InfraErrorRetryTimes int
}

type ctxTargetCalledCacheKey struct{}

type ctxForceNoRetryKey struct{}

// ctxCentralQuotaHeldKey 见 WithCtxCentralQuotaHeld。
type ctxCentralQuotaHeldKey struct{}

// ctxCentralAdmittedExptKey 见 WithCtxCentralAdmittedExpt。
type ctxCentralAdmittedExptKey struct{}

type ExptItemEvalEvent struct {
	SpaceID     int64
	ExptID      int64
	ExptRunID   int64
	ExptRunMode ExptRunMode

	EvalSetItemID               int64
	AsyncReportTrigger          bool
	AsyncEvaluatorReportTrigger bool

	CreateAt      int64
	RetryTimes    int
	MaxRetryTimes int
	Ext           map[string]string
	Session       *Session
}

func (e *ExptItemEvalEvent) WithCtxForceNoRetry(ctx context.Context) {
	ctxcache.Store(ctx, ctxForceNoRetryKey{}, struct{}{})
}

func (e *ExptItemEvalEvent) CtxForceNoRetry(ctx context.Context) bool {
	_, ok := ctxcache.Get[struct{}](ctx, ctxForceNoRetryKey{})
	return ok
}

func (e *ExptItemEvalEvent) IgnoreExistedTargetResult() bool {
	return e.ignoreExistedResult()
}

// WithCtxCentralAdmittedExpt 记录"该实验已通过中心调度准入检查、由本进程纳管"。
//
// 存在意义：准入判断（模式 / Scope 非空 / Scope 归属 / guard 已注入）在 item 锁**外**做 ——
// 不归本进程的消息不该先去抢锁；而取额度执行权必须在锁**内**做。两者被 item 锁隔成上下两层，
// 靠 ctx 把已查到的实验递下去，省掉锁内重复一次 GetByID（那是热路径上的一次多余 DB 往返，
// 且两次读之间实验可能已被改写，反而要额外推演一致性）。
//
// 只在 enforce 且通过**全部**准入检查后写入。缺失即表示"不走额度闸"，与 legacy 直通同义 ——
// 因此下游拿不到时必须直通而不是 fail-closed，否则 legacy 实验会被误挡。
func (e *ExptItemEvalEvent) WithCtxCentralAdmittedExpt(ctx context.Context, expt *Experiment) {
	ctxcache.Store(ctx, ctxCentralAdmittedExptKey{}, expt)
}

// CtxCentralAdmittedExpt 取回已准入的实验。ok=false 表示本条消息不经额度闸
// （legacy 实验，或已在准入层被丢弃）。
func (e *ExptItemEvalEvent) CtxCentralAdmittedExpt(ctx context.Context) (*Experiment, bool) {
	expt, ok := ctxcache.Get[*Experiment](ctx, ctxCentralAdmittedExptKey{})
	if !ok || expt == nil {
		return nil, false
	}
	return expt, true
}

// WithCtxCentralQuotaHeld 记录"本次处理已为该 item 持有一份中心调度额度预占"，值为账本 Scope。
//
// 存在意义：额度闸（HandleCentralReservation）在中间件链的内层，而"不可重试前置失败"的兜底
// 落 Fail 在更外层（HandleEventErr）。内层按 run log 状态判定时 item 还是 Processing、
// 因此正确地保留了 reservation；等外层把它改成 Fail，内层已经返回，再没有释放的机会。
// 靠 ctx 传递凭据，外层就能在落 Fail 之后补一次释放，而不必让 HandleEventErr 知道
// Scope / guard 这些中心调度概念。
//
// 只在真的取得执行权（ConfirmRunning 返回 true）之后调用 —— 否则外层会为一份并不存在的
// reservation 发起释放，虽然 Release 幂等不至于出错，但会掩盖"额度到底有没有预占"的真相。
func (e *ExptItemEvalEvent) WithCtxCentralQuotaHeld(ctx context.Context, schedulerScope string) {
	ctxcache.Store(ctx, ctxCentralQuotaHeldKey{}, schedulerScope)
}

// CtxCentralQuotaHeldScope 取回额度预占凭据。ok=false 表示本次处理没有持有预占
// （legacy 实验、被丢弃的消息、或额度闸压根没走到取执行权那一步）。
func (e *ExptItemEvalEvent) CtxCentralQuotaHeldScope(ctx context.Context) (string, bool) {
	scope, ok := ctxcache.Get[string](ctx, ctxCentralQuotaHeldKey{})
	if !ok || scope == "" {
		return "", false
	}
	return scope, true
}

func (e *ExptItemEvalEvent) WithCtxTargetCalled(ctx context.Context) {
	ctxcache.Store(ctx, ctxTargetCalledCacheKey{}, struct{}{})
}

func (e *ExptItemEvalEvent) CtxTargetCalled(ctx context.Context) bool {
	_, ok := ctxcache.Get[struct{}](ctx, ctxTargetCalledCacheKey{})
	return ok
}

func (e *ExptItemEvalEvent) IgnoreExistedEvaluatorResult(ctx context.Context) bool {
	if e.CtxTargetCalled(ctx) {
		return true
	}
	return e.ignoreExistedResult()
}

func (e *ExptItemEvalEvent) ignoreExistedResult() bool {
	return (e.ExptRunMode == EvaluationModeRetryItems || e.ExptRunMode == EvaluationModeRetryAll) && e.RetryTimes == 0
}

func (e *ExptItemEvalEvent) GetExptID() int64 {
	if e == nil {
		return 0
	}
	return e.ExptID
}

func (e *ExptItemEvalEvent) GetExptRunID() int64 {
	if e == nil {
		return 0
	}
	return e.ExptRunID
}

func (e *ExptItemEvalEvent) GetEvalSetItemID() int64 {
	if e == nil {
		return 0
	}
	return e.EvalSetItemID
}

type CalculateMode int

const (
	CreateAllFields        CalculateMode = 1
	UpdateSpecificField    CalculateMode = 2
	CreateAnnotationFields CalculateMode = 3
	UpdateAnnotationFields CalculateMode = 4
)

type AggrCalculateEvent struct {
	ExperimentID int64
	SpaceID      int64

	CalculateMode     CalculateMode
	SpecificFieldInfo *SpecificFieldInfo
}

type SpecificFieldInfo struct {
	FieldKey  string
	FieldType FieldType
}

func (e *AggrCalculateEvent) GetFieldKey() string {
	if e.SpecificFieldInfo == nil {
		return ""
	}

	return e.SpecificFieldInfo.FieldKey
}

func (e *AggrCalculateEvent) GetFieldType() FieldType {
	if e.SpecificFieldInfo == nil {
		return 0
	}

	return e.SpecificFieldInfo.FieldType
}

// OnlineExptTurnEvalResult 定义在线实验轮次评估结果结构体
type OnlineExptTurnEvalResult struct {
	EvaluatorVersionId int64              `json:"evaluator_version_id"`
	EvaluatorRecordId  int64              `json:"evaluator_record_id"`
	Score              float64            `json:"score"`
	Reasoning          string             `json:"reasoning"`
	Status             int32              `json:"status"`
	EvaluatorRunError  *EvaluatorRunError `json:"evaluator_run_error"`
	Ext                map[string]string  `json:"ext"`

	BaseInfo *BaseInfo `json:"base_info"`
}

// OnlineExptEvalResultEvent 定义在线实验评估结果事件结构体
type OnlineExptEvalResultEvent struct {
	ExptId          int64                       `json:"expt_id,omitempty"`
	TurnEvalResults []*OnlineExptTurnEvalResult `json:"turn_eval_results,omitempty"`
}

type EvaluatorRecordCorrectionEvent struct {
	EvaluatorResult    *EvaluatorResult  `json:"evaluator_result,omitempty"`
	EvaluatorRecordID  int64             `json:"evaluator_record_id"`
	EvaluatorVersionID int64             `json:"evaluator_version_id"`
	Ext                map[string]string `json:"ext,omitempty"`

	CreatedAt int64 `json:"created_at"`
	UpdatedAt int64 `json:"updated_at"`
}

type UpsertExptTurnResultFilterType string

const (
	UpsertExptTurnResultFilterTypeAuto   UpsertExptTurnResultFilterType = "auto"
	UpsertExptTurnResultFilterTypeCheck  UpsertExptTurnResultFilterType = "check"
	UpsertExptTurnResultFilterTypeManual UpsertExptTurnResultFilterType = "manual"
)

type ExptTurnResultFilterEvent struct {
	ExperimentID int64
	SpaceID      int64
	ItemID       []int64

	RetryTimes *int32
	FilterType *UpsertExptTurnResultFilterType
}

type ExportCSVEvent struct {
	ExportID     int64
	ExperimentID int64
	SpaceID      int64

	Session     *Session
	ExportScene ExportScene
	CreatedAt   int64
	// ExportColumns 与 ExportExptResultRequest.export_columns 一致；nil 表示全量列；非 nil 为白名单（子字段 nil/[] 均不导出该组）
	ExportColumns *ExptResultExportColumnSpec `json:"export_columns,omitempty"`
}

type ExptLifecycleEvent struct {
	ExptID        int64      `json:"expt_id"`
	ExptRunID     *int64     `json:"expt_run_id"`
	SpaceID       int64      `json:"space_id"`
	FromStatus    ExptStatus `json:"from_status"`
	ToStatus      ExptStatus `json:"to_status"`
	ExptType      ExptType   `json:"expt_type"`
	SourceType    SourceType `json:"source_type"`
	IdempotentKey string     `json:"idempotent_key,omitempty"`
}

type WebhookRetryEvent struct {
	ExptID     int64  `json:"expt_id"`
	SpaceID    int64  `json:"space_id"`
	DeliveryID string `json:"delivery_id"`
	WebhookURL string `json:"webhook_url"`
	Payload    string `json:"payload"`     // JSON string of the webhook body
	AttemptNum int    `json:"attempt_num"` // 当前第几次重试 (1/2/3)
	// Environment / Lane 携带首发时的环境语义，保证重试请求附加相同的泳道路由 header。
	// 旧消息（无此字段）反序列化后为 nil => 按 Prod 处理，向后兼容不 panic。
	Environment *WebhookEnvironment `json:"environment,omitempty"`
	Lane        *string             `json:"lane,omitempty"`
}

type ExportScene int

const (
	ExportSceneDefault         ExportScene = 0
	ExportSceneInsightAnalysis ExportScene = 1
)
