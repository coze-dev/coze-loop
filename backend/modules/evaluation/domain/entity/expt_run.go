// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"context"
	"strings"
	"time"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type ExptRunMode int32

const (
	EvaluationModeUnknown ExptRunMode = 0

	// EvaluationModeSubmit 创建后提交
	EvaluationModeSubmit ExptRunMode = 1

	// EvaluationModeFailRetry 失败后全部重试
	EvaluationModeFailRetry ExptRunMode = 2

	// EvaluationModeAppend 追加模式
	EvaluationModeAppend ExptRunMode = 3

	EvaluationModeRetryAll   ExptRunMode = 4
	EvaluationModeRetryItems ExptRunMode = 5
	// EvaluationModeTrialRun 试运行模式
	EvaluationModeTrialRun ExptRunMode = 6
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

func IsExptFinishing(status ExptStatus) bool {
	return status == ExptStatus_Terminating || status == ExptStatus_Draining
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
	defaultDaemonInterval        = 20 * time.Second
	defaultZombieIntervalSecond  = 60 * 60 * 36
	defaultItemEvalConcurNum     = 3
	defaultItemEvalInterval      = 20 * time.Second
	defaultSpaceExptConcurLimit  = 200
	defaultItemZombieSecond      = 60 * 20
	defaultItemAsyncZombieSecond = 60 * 60 * 3
	defaultMaxItemConcurNum      = 200

	// defaultEvalAsyncCtxTTLSecond 是 invoke_id → EvalAsyncCtx 的 Redis 存活时间兜底值 (12h)，
	// 与历史硬编码保持一致。真正生效的值见 GetEvalAsyncCtxTTL：它会跟随本空间的僵尸阈值抬高，
	// 避免出现「ctx 已过期但行还没被判僵尸」的真空区间 (上报报 not found，行继续挂着直到僵尸阈值)。
	defaultEvalAsyncCtxTTLSecond = 60 * 60 * 12
	// evalAsyncCtxTTLBufferSecond 是 TTL 相对僵尸阈值的安全余量。
	// 僵尸判定读的是 expt_item_result_run_log.updated_at (ON UPDATE CURRENT_TIMESTAMP)，
	// 计时会被行更新重置；而 Redis TTL 从 invoke 发出即定死、不续期。留一段余量让 TTL 稳定晚于僵尸判定。
	evalAsyncCtxTTLBufferSecond = 60 * 30
)

type ExptConsumerConf struct {
	ExptExecWorkerNum     int `json:"expt_exec_worker_num" mapstructure:"expt_exec_worker_num"`
	ExptItemEvalWorkerNum int `json:"expt_item_eval_worker_num" mapstructure:"expt_item_eval_worker_num"`

	ExptExecConf      *ExptExecConf           `json:"expt_exec_conf" mapstructure:"expt_exec_conf"`
	SpaceExptExecConf map[int64]*ExptExecConf `json:"space_expt_exec_conf" mapstructure:"space_expt_exec_conf"`

	SchedulerAbortCtrl *SchedulerAbortCtrl `json:"scheduler_abort_ctrl" mapstructure:"scheduler_abort_ctrl"`
}

// ExptTurnScoreHookConf 行维度得分 HTTP 回调配置。
// 命中时表示该实验的行维度得分需经外部 HTTP 接口计算，回调调用信息由 URL/Method/TimeoutMS 描述。
type ExptTurnScoreHookConf struct {
	URL       string `json:"url" mapstructure:"url"`
	Method    string `json:"method" mapstructure:"method"`
	TimeoutMS int64  `json:"timeout_ms" mapstructure:"timeout_ms"`
}

// CaseScoreItem 为单个评估器在该行的得分，对应 score_api.md 中 /score/case 的 evaluator_score 元素，
// 并额外携带 ExptID/EvaluatorID/EvaluatorVersionID 信息。
type CaseScoreItem struct {
	EvaluatorName      string  `json:"evaluator_name"`
	EvaluatorID        int64   `json:"evaluator_id"`
	EvaluatorVersionID int64   `json:"evaluator_version_id"`
	Score              float64 `json:"score"`
}

// CaseScoreRequest 为 /score/case 的请求体。
type CaseScoreRequest struct {
	ExptID         int64            `json:"expt_id"`
	EvaluatorScore []*CaseScoreItem `json:"evaluator_score"`
}

// CaseScoreResponse 为 /score/case 的响应体。即使打分异常，HTTP 仍返回 200，
// score 为兜底值并在 error 字段给出说明，调用方需同时检查 error。
type CaseScoreResponse struct {
	Score float64 `json:"score"`
	Error string  `json:"error"`
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

func (e *ExptConsumerConf) GetSchedulerAbortCtrl() *SchedulerAbortCtrl {
	if e != nil && e.SchedulerAbortCtrl != nil {
		return e.SchedulerAbortCtrl
	}
	return nil
}

type SchedulerAbortCtrl struct {
	UserExptTypeCtrl  map[string][]ExptType `json:"user_expt_type_ctrl" mapstructure:"user_expt_type_ctrl"`
	SpaceExptTypeCtrl map[int64][]ExptType  `json:"space_expt_type_ctrl" mapstructure:"space_expt_type_ctrl"`
	ExptIDCtrl        map[int64]bool        `json:"expt_id_ctrl" mapstructure:"expt_id_ctrl"`
}

func (s *SchedulerAbortCtrl) Abort(spaceID, exptID int64, userID string, exptType ExptType) bool {
	if s == nil {
		return false
	}

	if s.ExptIDCtrl != nil {
		if abort, exists := s.ExptIDCtrl[exptID]; exists && abort {
			return true
		}
	}

	if s.SpaceExptTypeCtrl != nil {
		if exptTypes, exists := s.SpaceExptTypeCtrl[spaceID]; exists {
			for _, et := range exptTypes {
				if et == exptType {
					return true
				}
			}
		}
	}

	if s.UserExptTypeCtrl != nil {
		if exptTypes, exists := s.UserExptTypeCtrl[userID]; exists {
			for _, et := range exptTypes {
				if et == exptType {
					return true
				}
			}
		}
	}

	return false
}

type ExptExecConf struct {
	DaemonIntervalSecond int `json:"daemon_interval_second" mapstructure:"daemon_interval_second"`
	ZombieIntervalSecond int `json:"expt_zombie_second" mapstructure:"expt_zombie_second"`
	SpaceExptConcurLimit int `json:"space_expt_concur_limit" mapstructure:"space_expt_concur_limit"`

	ExptItemEvalConf *ExptItemEvalConf `json:"expt_item_eval_conf" mapstructure:"expt_item_eval_conf"`

	// RetryYield 失败重试"让位降权"改造的灰度开关；nil / 默认关 → 保持改造前行为(执行侧重投 MQ + id asc 排序)。
	// 值在实验运行发起时读一次并固化进 ExptScheduleEvent.Ext, 运行中不再现读, 详见技术方案 §9.3.1。
	RetryYield *RetryYieldConf `json:"retry_yield" mapstructure:"retry_yield"`
}

// RetryYieldConf 让位降权改造的灰度开关：全局布尔 + 空间白名单两级。
// RetryYieldExtKey 让位降权灰度开关值在 ExptScheduleEvent.Ext / ExptItemEvalEvent.Ext 中的键。
// 运行发起时把开关值(configer 读取结果)写入该键, 消费侧读事件里的值而非现读配置(按 expt_run_id 固化, §9.3.1)。
const RetryYieldExtKey = "retry_yield_enabled"

type RetryYieldConf struct {
	// Enabled 全局开关；true → 所有空间生效(白名单被忽略)。
	Enabled bool `json:"enabled" mapstructure:"enabled"`
	// SpaceIDs 空间级灰度白名单；仅在 Enabled=false 时生效, 命中即开。
	SpaceIDs []int64 `json:"space_ids" mapstructure:"space_ids"`
	// IndexReady 声明降权索引 idx_expt_run_retry_pick 是否已在该环境的库上建成。
	//
	// ⚠️ 与 Enabled/SpaceIDs 语义不同, 这不是业务灰度而是 schema 事实声明, 故:
	//   - 只影响执行计划(是否下 ForceIndex hint), 不改任何业务语义;
	//   - 不随 expt_run 固化(不进 event.Ext), 每次挑选现读配置 —— 索引建成后翻此开关立即生效,
	//     无需等运行中的实验跑完或重跑。
	//
	// false(默认): 不下 ForceIndex, 由优化器自选(实测退化为 filesort)。排序语义不变,
	//   仍是 retry_times asc, id asc, 让位降权功能完整, 仅失去索引序 + LIMIT 提前停止。
	// true: 下 ForceIndex(idx_expt_run_retry_pick) 取最优执行计划。
	//   ★ 索引未建成时置 true 会让挑选直接报 Key doesn't exist(ForceIndex 不静默降级), 故务必先建索引再翻。
	//
	// 为何默认 false: 线上 expt_item_result_run_log 是高频写入热表(实测 CN 某库 99.6M 行 / 36.1GB,
	// 日均新增约 830 万行), 加 6 列复合索引的构建期风险(online log 溢出致 DDL 失败)与长期写放大
	// 高于 filesort 的代价, 故先只加列、索引后补。详见技术方案索引选型节。
	IndexReady bool `json:"index_ready" mapstructure:"index_ready"`
}

// IsIndexReady 判断降权索引是否已建成(决定是否下 ForceIndex hint)。
// nil → false, 保守走优化器自选, 绝不会因配置缺失而报 Key doesn't exist。
func (c *RetryYieldConf) IsIndexReady() bool {
	return c != nil && c.IndexReady
}

// IsSpaceEnabled 判断该空间是否开启让位降权。
// nil → 关闭；Enabled=true → 全局开；否则命中 SpaceIDs 才开。
func (c *RetryYieldConf) IsSpaceEnabled(spaceID int64) bool {
	if c == nil {
		return false
	}
	if c.Enabled {
		return true
	}
	for _, id := range c.SpaceIDs {
		if id == spaceID {
			return true
		}
	}
	return false
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

// GetRetryYieldConf nil-safe 取让位降权灰度开关配置; 未配置返回 nil(IsSpaceEnabled 对 nil 返回 false)。
func (e *ExptExecConf) GetRetryYieldConf() *RetryYieldConf {
	if e != nil {
		return e.RetryYield
	}
	return nil
}

type ExptItemEvalConf struct {
	ConcurNum         int `json:"concur_num" mapstructure:"concur_num"`
	IntervalSecond    int `json:"interval_second" mapstructure:"interval_second"`
	ZombieSecond      int `json:"zombie_second" mapstructure:"zombie_second"`
	AsyncZombieSecond int `json:"async_zombie_second" mapstructure:"async_zombie_second"`
	MaxItemConcurNum  int `json:"max_item_concur_num" mapstructure:"max_item_concur_num"`
	// EvalAsyncCtxTTLSecond 显式指定 invoke_id → EvalAsyncCtx 的 Redis TTL (秒)。
	// 留空 (0) 时不需要单独配：GetEvalAsyncCtxTTL 会自动按 async_zombie_second 推导，
	// 只在需要脱离僵尸阈值单独控制 ctx 存活时长时才填。
	EvalAsyncCtxTTLSecond int `json:"eval_async_ctx_ttl_second" mapstructure:"eval_async_ctx_ttl_second"`
}

func (e *ExptItemEvalConf) GetConcurNum() int {
	if e != nil && e.ConcurNum > 0 {
		return e.ConcurNum
	}
	return defaultItemEvalConcurNum
}

func (e *ExptItemEvalConf) GetMaxItemConcurNum() int {
	if e != nil && e.MaxItemConcurNum > 0 {
		return e.MaxItemConcurNum
	}
	return defaultMaxItemConcurNum
}

func (e *ExptItemEvalConf) GetInterval() time.Duration {
	if e != nil && e.IntervalSecond > 0 {
		return time.Duration(e.IntervalSecond) * time.Second
	}
	return defaultItemEvalInterval
}

func (e *ExptItemEvalConf) getZombieSecond() int {
	if e != nil && e.ZombieSecond > 0 {
		return e.ZombieSecond
	}
	return defaultItemZombieSecond
}

func (e *ExptItemEvalConf) getAsyncZombieSecond() int {
	if e != nil && e.AsyncZombieSecond > 0 {
		return e.AsyncZombieSecond
	}
	return defaultItemAsyncZombieSecond
}

func (e *ExptItemEvalConf) GetItemZombieSecond(isAsync bool) int {
	if isAsync {
		return e.getAsyncZombieSecond()
	}
	return e.getZombieSecond()
}

// GetEvalAsyncCtxTTL 返回 invoke_id → EvalAsyncCtx 的 Redis TTL。
//
// 取值优先级：
//  1. 显式配置 eval_async_ctx_ttl_second（需要脱离僵尸阈值单独控制时用）
//  2. 否则取 max(async_zombie_second + buffer, 12h 兜底)
//
// 之所以要跟 async_zombie_second 挂钩：这两个值粒度不同，容易配出真空区间。
// TTL 是「单次 invoke」维度、从 AsyncInvoke 发出即定死、不续期；
// async_zombie_second 是「item 行」维度、基准是 updated_at (ON UPDATE CURRENT_TIMESTAMP)、
// 会被任何一次行更新重置。TTL 一旦短于僵尸阈值，用户在中间区段上报会拿到
// eval async context not found，而行本身还是 Processing、要一直挂到僵尸阈值才失败。
// 所以这里让 TTL 始终不低于僵尸阈值 + 余量，把那段真空消掉。
func (e *ExptItemEvalConf) GetEvalAsyncCtxTTL() time.Duration {
	if e != nil && e.EvalAsyncCtxTTLSecond > 0 {
		return time.Duration(e.EvalAsyncCtxTTLSecond) * time.Second
	}

	ttlSecond := e.getAsyncZombieSecond() + evalAsyncCtxTTLBufferSecond
	if ttlSecond < defaultEvalAsyncCtxTTLSecond {
		ttlSecond = defaultEvalAsyncCtxTTLSecond
	}
	return time.Duration(ttlSecond) * time.Second
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

	// ItemConfig 单行执行的唯一配置源(仅 MultiSetConfig 新实验类型)。来自 expt_item_ref.item_config,
	// 含该 item 行级的 EvaluatorConfs (含 alias / filter / FilterMode) 和 EvalTargetConf。
	// 老实验类型 (DataSet) 或读取失败时为 nil, 执行侧据此回退到 expt 级 EvaluatorsConf 老路径。
	ItemConfig *ExptItemConfig

	// EvalSetVersionID 该 item 归属评测集的版本 ID (per-item)。多评测集下各 item 可属不同集/版本,
	// 由 BuildExptRecordEvalCtx 从 expt_item_ref 解析后回填; 单评测集/老实验为实验主集版本。
	// EvalSetItem.EvaluationSetID 是归属集 ID, 二者配合可定位该 item 的 (集, 版本), 供下游事件组装。
	EvalSetVersionID int64
}

// EvalSetSourceSpaceID 该行评测集来源空间: 多集从 ItemConfig(行级冻结), 单集从 Expt 冻结列; 0=同调用方空间。
// ★ 多集(ItemConfig != nil)时行级冻结值即权威, 0 表示"该集在调用方空间"而非"未设置", 故不可再回退顶层列:
// 顶层 EvalSetSpaceID 会被 configs[0] 兜底回填成主集来源空间, 混合空间多集(部分集跨空间/部分集同空间)下
// 回退会让同空间集被错送到主集来源空间读 → BatchGetEvaluationSetItems 查不到 → 该集 item 全失败且重试无效。
func (e *ExptItemEvalCtx) EvalSetSourceSpaceID() int64 {
	if e == nil {
		return 0
	}
	if e.ItemConfig != nil {
		return e.ItemConfig.EvalSetSourceSpaceID
	}
	if e.Expt != nil {
		return e.Expt.EvalSetSpaceID
	}
	return 0
}

// TargetSourceSpaceID 该行评测对象来源空间: 多集从 ItemConfig, 单集从 Expt 冻结列; 0=同调用方空间。
// ★ 与 EvalSetSourceSpaceID 的语义不对称, 勿"顺手统一": 多集执行恒用顶层 GLOBAL target
// (见 expt_manage_impl 多集 per-set target 鉴权处注释), per-set 的 TargetSourceSpaceID
// 取自 setConf.TargetSpaceID, 未配 per-set target 时为 0 表示"未设置"而非"target 在调用方空间"。
// 故此处保留对顶层列的回退: 若按 0 直接返回, 顶层跨空间 target 会被按调用方空间加载
// → 601203004 resource not found, target 调用失败(实测 EXP 7590113853734093570 两条 item 全挂)。
func (e *ExptItemEvalCtx) TargetSourceSpaceID() int64 {
	if e == nil {
		return 0
	}
	if e.ItemConfig != nil && e.ItemConfig.TargetSourceSpaceID > 0 {
		return e.ItemConfig.TargetSourceSpaceID
	}
	if e.Expt != nil {
		return e.Expt.TargetSpaceID
	}
	return 0
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

	defer func() { logs.CtxInfo(ctx, "GetTurnEvalLogID with log_id: %v", logID) }()

	if turnRunLog == nil {
		return logs.NewLogID()
	}

	if len(turnRunLog.LogID) == 0 {
		turnRunLog.LogID = logs.NewLogID()
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
	EvaluatorResults []*EvaluatorRecord // slice, not map — supports alias multi-instances of same versionID
	EvalErr          error
	AsyncAbort       bool
}

func (e *ExptTurnRunResult) GetTargetResult() *EvalTargetRecord {
	if e != nil {
		return e.TargetResult
	}
	return nil
}

func (e *ExptTurnRunResult) SetTargetResult(er *EvalTargetRecord) *ExptTurnRunResult {
	e.TargetResult = er
	return e
}

func (e *ExptTurnRunResult) SetEvaluatorResults(er []*EvaluatorRecord) *ExptTurnRunResult {
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
	for _, r := range e.EvaluatorResults {
		if r != nil && r.EvaluatorVersionID == evaluatorVersionID {
			return r
		}
	}
	return nil
}

// GetEvaluatorRecordByVerAlias 按 (versionID, alias) 双键定位 record。新实验类型 (MultiSetConfig)
// 下同一 versionID 可能有多个 alias 实例, 老的 GetEvaluatorRecord 只按 versionID 查找会返回首个匹配,
// 在 alias 多实例场景下会导致后续实例被"误判为已完成"而跳过执行。
// 老实验类型 (alias 恒空) 调用本方法和 GetEvaluatorRecord 等价。
func (e *ExptTurnRunResult) GetEvaluatorRecordByVerAlias(evaluatorVersionID int64, alias string) *EvaluatorRecord {
	if e == nil {
		return nil
	}
	for _, r := range e.EvaluatorResults {
		if r != nil && r.EvaluatorVersionID == evaluatorVersionID && r.Alias == alias {
			return r
		}
	}
	return nil
}

func (e *ExptTurnRunResult) AbortWithTargetResult(expt *Experiment) bool {
	// invalid target result
	if e.TargetResult == nil {
		e.SetEvalErr(errorx.NewByCode(errno.CommonInternalErrorCode, errorx.WithExtraMsg("target result is nil")))
		return true
	}

	// target exec error
	if e.TargetResult.EvalTargetOutputData != nil && e.TargetResult.EvalTargetOutputData.EvalTargetRunError != nil {
		return true
	}

	// target async exec, with no record
	if expt.AsyncCallTarget() && gptr.Indirect(e.TargetResult.Status) == EvalTargetRunStatusAsyncInvoking {
		e.AsyncAbort = true
		return true
	}

	return false
}

func (e *ExptTurnRunResult) AbortWithEvaluatorResults(ctx context.Context, event *ExptItemEvalEvent) bool {
	// evaluator async exec, check if any evaluator is in async invoking status
	for _, record := range e.EvaluatorResults {
		if record != nil && record.Status == EvaluatorRunStatusAsyncInvoking {
			e.AsyncAbort = true
			event.WithCtxForceNoRetry(ctx)
			return true
		}
	}
	return false
}

//go:generate  mockgen -destination  ./mocks/expt_scheduler_mock.go  --package mocks . ExptSchedulerMode
type ExptSchedulerMode interface {
	Mode() ExptRunMode
	ExptStart(ctx context.Context, event *ExptScheduleEvent, expt *Experiment) error
	ScanEvalItems(ctx context.Context, event *ExptScheduleEvent, expt *Experiment) (toSubmit, incomplete, complete []*ExptEvalItem, err error)
	ExptEnd(ctx context.Context, event *ExptScheduleEvent, expt *Experiment, toSubmit, incomplete int) (nextTick bool, err error)
	ScheduleStart(ctx context.Context, event *ExptScheduleEvent, expt *Experiment) error
	NextTick(ctx context.Context, event *ExptScheduleEvent, nextTick bool) error
	PublishResult(ctx context.Context, turnEvaluatorRefs []*ExptTurnEvaluatorResultRef, event *ExptScheduleEvent) error
}

type CKDBConfig struct {
	ExptTurnResultFilterDBName string `json:"expt_turn_result_filter_db_name" mapstructure:"expt_turn_result_filter_db_name"`
	DatasetItemsSnapshotDBName string `json:"dataset_items_snapshot_db_name" mapstructure:"dataset_items_snapshot_db_name"`
}

type EvalAsyncCtx struct {
	Event                   *ExptItemEvalEvent
	RecordID                int64
	AsyncUnixMS             int64 // async call time with unix ms ts
	Session                 *Session
	Callee                  string
	EvaluatorVersionID      int64 // evaluator version id, used for evaluator async scenario
	EnableExtractTrajectory *bool
	ResumeReady             bool   `json:"resume_ready,omitempty"` // experiment turn refs are durable and callback may resume scheduling
	CallbackURL             string `json:"callback_url,omitempty"` // 异步执行完成后回调通知的 URL，为空则不回调
	// 下述字段用于沙箱内部 step 上报的 tag 反查, 由 target async 写入位点从 etec 填充,
	// 调试场景 (无实验上下文) 保留零值, 由上报侧回退为占位符.
	TargetID         int64
	DatasetID        int64
	DatasetVersionID int64
	ItemKey          string
	DatasetKey       string
	// AgentName 沙箱 agent 应用名称 (SandboxAgent.Name), 供 invoke_finished 打点复用。
	AgentName string
	// ApplicationID 沙箱 agent 应用 id (即 EvalTarget.SourceTargetID, AgentKit application_id), 供 invoke_finished 打点复用。
	ApplicationID string
}
