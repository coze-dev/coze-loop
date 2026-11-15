// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"strconv"
	"strings"
	"time"
)

// CentralResourceKind 中心调度资源种类。
//
// 注意：本题禁止新增数据库表/列/索引，资源额度只存在于 Redis 中心账本中，
// 不落 MySQL。资源种类名称同时作为 Redis hash field，新增种类时只允许追加。
type CentralResourceKind string

const (
	// CentralResourceItemSlot 单个评测 item 的执行槽位（一次 item 派发占 1）。
	CentralResourceItemSlot CentralResourceKind = "item_slot"
	// CentralResourceEvaluatorSlot 单次 item 执行涉及的评估器槽位（按评估器数量占）。
	CentralResourceEvaluatorSlot CentralResourceKind = "evaluator_slot"
	// CentralResourceSandboxExecute 沙箱 execute 槽位（SandboxAgent 评测对象按一次 item 占 1）。
	CentralResourceSandboxExecute CentralResourceKind = "sandbox_execute"
)

// DefaultCentralResourceKinds 是中心账本默认参与原子预占的资源种类顺序。
var DefaultCentralResourceKinds = []CentralResourceKind{
	CentralResourceItemSlot,
	CentralResourceEvaluatorSlot,
	CentralResourceSandboxExecute,
}

// CentralResourceClaim 一类资源的预占量。
type CentralResourceClaim struct {
	Kind   CentralResourceKind `json:"kind"`
	Amount int64               `json:"amount"`
}

// CentralLeaseState 是 lease 的唯一状态。
//
// 一次预占在任意时刻只允许一个状态；状态迁移由 Redis Lua CAS 完成，
// 不允许出现“成功 + 失败”这类并存的复合状态。
type CentralLeaseState string

const (
	// CentralLeaseStateReserved 已预占、尚未确认派发成功。
	CentralLeaseStateReserved CentralLeaseState = "reserved"
	// CentralLeaseStateDispatched 派发明确成功，item 已进入执行。
	CentralLeaseStateDispatched CentralLeaseState = "dispatched"
	// CentralLeaseStatePendingConfirm 派发结果不明（超时/异常），等待 deadline sweeper 确认或释放。
	CentralLeaseStatePendingConfirm CentralLeaseState = "pending_confirm"
)

// CentralLease 一次 item 派发的多资源预占凭据。
//
// LeaseID 由 exptID/runID/itemID/attempt 确定性生成：每次重试产生新的 attempt，
// 因此同一 item 的两次执行绝不会复用同一个预占身份。释放按
// (exptID, runID, itemID) 匹配全部 attempt，天然幂等。
type CentralLease struct {
	LeaseID    string                 `json:"lease_id"`
	SpaceID    int64                  `json:"space_id"`
	ExptID     int64                  `json:"expt_id"`
	ExptRunID  int64                  `json:"expt_run_id"`
	ItemID     int64                  `json:"item_id"`
	Attempt    int64                  `json:"attempt"`
	State      CentralLeaseState      `json:"state"`
	Claims     []CentralResourceClaim `json:"claims"`
	AcquiredAt int64                  `json:"acquired_at"`
	ExpireAt   int64                  `json:"expire_at"`
}

// TotalClaim 返回指定资源类别的预占量；不存在时返回 0。
func (l *CentralLease) TotalClaim(kind CentralResourceKind) int64 {
	if l == nil {
		return 0
	}
	for _, claim := range l.Claims {
		if claim.Kind == kind {
			return claim.Amount
		}
	}
	return 0
}

// CentralActiveRun 活跃实验运行（中心 tick 的候选集合成员）。
type CentralActiveRun struct {
	SpaceID       int64  `json:"space_id"`
	ExptID        int64  `json:"expt_id"`
	ExptRunID     int64  `json:"expt_run_id"`
	Priority      int    `json:"priority"`
	Seq           int64  `json:"seq"`
	RegisteredAt  int64  `json:"registered_at"`
	SessionUserID string `json:"session_user_id"`
}

// CentralRunStatus 区分“待处理的初始化”与“已活跃的维护”，两者分属不同 key，互不覆盖。
type CentralRunStatus string

const (
	// CentralRunStatusPending 首次登记、尚未确认任何 item 派发成功。
	CentralRunStatusPending CentralRunStatus = "pending"
	// CentralRunStatusActive 至少一次 item 派发已确认成功，进入常规维护。
	CentralRunStatusActive CentralRunStatus = "active"
)

// CentralRunState 是一次实验运行的冻结调度状态。
//
// 实验一旦进入中心调度，mode/priority/targetConcurrency 即写入该记录并冻结；
// 后续 tick（含重试 item）只读该记录，不再重新解析 ext/TCC。
type CentralRunState struct {
	SpaceID           int64                `json:"space_id"`
	ExptID            int64                `json:"expt_id"`
	ExptRunID         int64                `json:"expt_run_id"`
	Mode              CentralSchedulerMode `json:"mode"`
	Priority          int                  `json:"priority"`
	Seq               int64                `json:"seq"`
	TargetConcurrency int                  `json:"target_concurrency"`
	Status            CentralRunStatus     `json:"status"`
	FrozenAt          int64                `json:"frozen_at"`
	RegisteredAt      int64                `json:"registered_at"`
	SwitchTicketID    string               `json:"switch_ticket_id,omitempty"`
	SwitchOperator    string               `json:"switch_operator,omitempty"`
	SwitchReason      string               `json:"switch_reason,omitempty"`
	SwitchedAt        int64                `json:"switched_at,omitempty"`
}

// CentralModeSwitchAuthorization 是实验负责人提工单、审批后写入 TCC 白名单的一次性授权记录。
//
// 研发不得自行改口径：这里只消费审批结果，不提供写入口。一个实验只允许切换一次，
// 消费标记写入 qn:central: 前缀的 Redis key，并在 run state 上记录操作人与原因。
type CentralModeSwitchAuthorization struct {
	ToMode     string `json:"to_mode" mapstructure:"to_mode"`
	TicketID   string `json:"ticket_id" mapstructure:"ticket_id"`
	Operator   string `json:"operator" mapstructure:"operator"`
	Reason     string `json:"reason" mapstructure:"reason"`
	ApprovedAt int64  `json:"approved_at" mapstructure:"approved_at"`
}

// CentralLedgerDrift 描述一笔可定位的账本漂移。
type CentralLedgerDrift struct {
	LeaseID string              `json:"lease_id"`
	Kind    CentralResourceKind `json:"kind,omitempty"`
	Amount  int64               `json:"amount,omitempty"`
	Usage   int64               `json:"usage,omitempty"`
	Reason  string              `json:"reason"`
}

// CentralEnvScope 中心账本的所有权命名空间。
//
// 线上与 PPE 泳道共用 Redis，因此所有 center key 都必须带上 Env + Lane，
// 完整 key 统一以 qn:central: 开头，禁止读写其它前缀。
type CentralEnvScope struct {
	Env  string `json:"env"`
	Lane string `json:"lane"`
}

// IsValid 判断环境/泳道是否可安全用于账本读写。
//
// 整条链路（创建、扫描、租约、账本、消费、恢复、查询）都必须先通过该检查；
// 任一维度为空即按失败处理，绝不回落到 unknown/default 去借用别人的数据。
func (s CentralEnvScope) IsValid() bool {
	return strings.TrimSpace(s.Env) != "" && strings.TrimSpace(s.Lane) != ""
}

// Sanitize 将环境/泳道名转换成 key 安全片段：只保留字母数字、下划线、连字符，其余替换为 '_'。
// 空值原样保留为空串，由 IsValid 判定为不可用；这里不做任何“借用默认环境”的回落。
func (s CentralEnvScope) Sanitize() CentralEnvScope {
	return CentralEnvScope{Env: sanitizeKeyPart(s.Env), Lane: sanitizeKeyPart(s.Lane)}
}

func sanitizeKeyPart(in string) string {
	in = strings.TrimSpace(in)
	if in == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(in))
	for _, c := range in {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '_', c == '-':
			b.WriteRune(c)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}

// 中心调度扩展位。优先级与模式均复用既有 experiment.eval_conf.ext，
// 不新增数据库列，也不新增 IDL 字段，保证旧调用方契约不变。
const (
	// CentralExtKeySchedulingMode 实验级中心调度模式；缺省时回落到 TCC 默认模式。
	CentralExtKeySchedulingMode = "central_scheduling_mode"
	// CentralExtKeyPriority 实验级调度优先级；缺省时回落到 TCC 默认值。
	CentralExtKeyPriority = "central_priority"
	// CentralExtKeyTargetConcurrency 实验级目标并发；缺省时按“空间级 > 全局 > legacy 扫描批量”回落。
	CentralExtKeyTargetConcurrency = "central_target_concurrency"
)

// CentralDegradedPolicy 是资源池降级时的明确口径；默认 block_new，不允许静默放量。
type CentralDegradedPolicy string

const (
	// CentralDegradedPolicyBlockNew 降级期间暂停所有新的预占。
	CentralDegradedPolicyBlockNew CentralDegradedPolicy = "block_new"
	// CentralDegradedPolicyReduceCapacity 降级期间按配置比例下调容量。
	CentralDegradedPolicyReduceCapacity CentralDegradedPolicy = "reduce_capacity"
)

// CentralResourceConfig 单类资源的生产配置。容量只来自配置，代码内不提供生产默认值。
type CentralResourceConfig struct {
	Capacity int64 `json:"capacity" mapstructure:"capacity"`
	Enabled  bool  `json:"enabled" mapstructure:"enabled"`
}

// 数值口径默认值（已与需求方确认）。
const (
	DefaultCentralAgingWindowTicks      = 15
	DefaultCentralAgingStep             = 10
	DefaultCentralHighestPriority       = 1
	DefaultCentralDispatchDeadlineTicks = 2
	DefaultCentralDegradedRatio         = 0.5
	DefaultCentralDegradedRecoverTicks  = 3
	DefaultCentralPendingTTLSecond      = 600
)

// CentralSchedulerMode 中心调度模式。
type CentralSchedulerMode string

const (
	// CentralSchedulerModeLegacy 旧模式：保留原有每实验 MQ 自循环与每空间 200 运行准入，不做任何中心调度。
	CentralSchedulerModeLegacy CentralSchedulerMode = "legacy"
	// CentralSchedulerModeEnforce 新模式：由中心 tick 驱动调度，派发前经中心账本原子预占。
	CentralSchedulerModeEnforce CentralSchedulerMode = "enforce"
)

// DefaultCentralPriority 是未显式配置优先级时的默认值（数值越小越优先）。
const DefaultCentralPriority = 100

// ParseCentralSchedulerMode 解析模式字符串；无法识别时返回 legacy，保证未知配置不会意外打开新模式。
func ParseCentralSchedulerMode(raw string) CentralSchedulerMode {
	switch CentralSchedulerMode(strings.TrimSpace(strings.ToLower(raw))) {
	case CentralSchedulerModeEnforce:
		return CentralSchedulerModeEnforce
	default:
		return CentralSchedulerModeLegacy
	}
}

// ParseCentralPriority 解析优先级；非法值返回默认优先级。
func ParseCentralPriority(raw string, fallback int) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return v
}

// CentralSchedulerConfig 中心调度配置（TCC/静态配置）。
//
// 默认 Enabled=false，即不配置时完全保持旧行为。
type CentralSchedulerConfig struct {
	Enabled bool `json:"enabled" mapstructure:"enabled"`
	// DefaultMode 未在实验 ext 中显式指定时的默认模式；缺省 legacy。
	DefaultMode string `json:"default_mode" mapstructure:"default_mode"`
	// EnforceSpaceIDs 显式打开 enforce 的空间白名单（即使 DefaultMode=legacy 也生效）。
	EnforceSpaceIDs []int64 `json:"enforce_space_ids" mapstructure:"enforce_space_ids"`
	// TickIntervalSecond 中心 tick 周期（秒），缺省 20。
	TickIntervalSecond int `json:"tick_interval_second" mapstructure:"tick_interval_second"`
	// LeaseTTLSecond 预占 lease 的 TTL（秒），缺省 300；必须大于 tick 周期。
	LeaseTTLSecond int `json:"lease_ttl_second" mapstructure:"lease_ttl_second"`
	// Resources 是资源池的唯一容量来源；未出现的 kind 视为未配置/禁用，不派发相关实验。
	Resources map[string]CentralResourceConfig `json:"resources" mapstructure:"resources"`
	// Capacities 是旧版扁平容量配置，仅为已有 TCC 配置向后兼容保留。
	// 非正值同样视为未配置；新部署应使用 Resources 以表达 enabled=false。
	Capacities map[string]int64 `json:"capacities" mapstructure:"capacities"`

	// DefaultTargetConcurrency 全局默认目标并发；<=0 表示不启用缺口补位（沿用 legacy 扫描批量）。
	DefaultTargetConcurrency int `json:"default_target_concurrency" mapstructure:"default_target_concurrency"`
	// SpaceTargetConcurrency 按空间覆盖目标并发（key 为 spaceID 字符串）。
	SpaceTargetConcurrency map[string]int `json:"space_target_concurrency" mapstructure:"space_target_concurrency"`

	// AgingWindowTicks 防饿死抬升窗口（tick 数），缺省 15。
	AgingWindowTicks int `json:"aging_window_ticks" mapstructure:"aging_window_ticks"`
	// AgingStep 每个窗口提升的优先级数值，缺省 10。
	AgingStep int `json:"aging_step" mapstructure:"aging_step"`
	// HighestPriority 抬升上限（数值最小），缺省 1，且不会覆盖冻结的 base priority。
	HighestPriority int `json:"highest_priority" mapstructure:"highest_priority"`

	// Degraded 资源池是否处于降级；恢复需连续 DegradedRecoverTicks 个正常 tick。
	Degraded bool `json:"degraded" mapstructure:"degraded"`
	// DegradedPolicy block_new（默认）或 reduce_capacity。
	DegradedPolicy string `json:"degraded_policy" mapstructure:"degraded_policy"`
	// DegradedRatio reduce_capacity 时的容量比例，缺省 0.5；只降不升。
	DegradedRatio float64 `json:"degraded_ratio" mapstructure:"degraded_ratio"`
	// DegradedRecoverTicks 连续正常 tick 数达到后恢复，缺省 3。
	DegradedRecoverTicks int `json:"degraded_recover_ticks" mapstructure:"degraded_recover_ticks"`

	// DispatchDeadlineTicks pending_confirm 的超时 tick 数，缺省 2。
	DispatchDeadlineTicks int `json:"dispatch_deadline_ticks" mapstructure:"dispatch_deadline_ticks"`
	// PendingTTLSecond 待处理初始化的保留时长（秒），缺省 600。
	PendingTTLSecond int `json:"pending_ttl_second" mapstructure:"pending_ttl_second"`

	// ModeSwitchAuthorizations 是审批后的模式切换白名单，key 为 exptID 字符串。
	// 由实验负责人提工单、审批后写入；研发只消费，不自行修改口径。
	ModeSwitchAuthorizations map[string]CentralModeSwitchAuthorization `json:"mode_switch_authorizations" mapstructure:"mode_switch_authorizations"`
}

// DefaultCentralCapacities 仅用于单元测试/离线替身构造显式配置，生产路径不会自动套用。
var DefaultCentralCapacities = map[CentralResourceKind]int64{
	CentralResourceItemSlot:       200,
	CentralResourceEvaluatorSlot:  600,
	CentralResourceSandboxExecute: 200,
}

// GetTickInterval 返回 tick 周期。
func (c *CentralSchedulerConfig) GetTickInterval() time.Duration {
	if c != nil && c.TickIntervalSecond > 0 {
		return time.Duration(c.TickIntervalSecond) * time.Second
	}
	return 20 * time.Second
}

// GetLeaseTTL 返回 lease TTL；至少是 tick 周期的 2 倍，避免一个 tick 还没跑完 lease 就过期。
func (c *CentralSchedulerConfig) GetLeaseTTL() time.Duration {
	ttl := 300 * time.Second
	if c != nil && c.LeaseTTLSecond > 0 {
		ttl = time.Duration(c.LeaseTTLSecond) * time.Second
	}
	if min := c.GetTickInterval() * 2; ttl < min {
		ttl = min
	}
	return ttl
}

// GetCapacities 返回当前生效的资源容量。
//
// 容量完全来自配置：未配置、capacity<=0、enabled=false 的 kind 不会出现在返回值里，
// 调度侧据此判定该资源不可用并停止派发相关实验。这里不提供任何生产默认容量。
func (c *CentralSchedulerConfig) GetCapacities() map[CentralResourceKind]int64 {
	out := make(map[CentralResourceKind]int64)
	if c == nil {
		return out
	}
	for rawKind, rawCapacity := range c.Capacities {
		if rawCapacity <= 0 {
			continue
		}
		out[CentralResourceKind(rawKind)] = rawCapacity
	}
	for rawKind, resource := range c.Resources {
		kind := CentralResourceKind(rawKind)
		if !resource.Enabled || resource.Capacity <= 0 {
			// 显式 resources 配置覆盖旧 capacities：disabled 表示该类资源整体不可用。
			delete(out, kind)
			continue
		}
		out[kind] = resource.Capacity
	}
	return out
}

// GetEffectiveCapacities 在降级口径下计算实际可用容量，绝不高于配置容量。
func (c *CentralSchedulerConfig) GetEffectiveCapacities() map[CentralResourceKind]int64 {
	return c.GetEffectiveCapacitiesWithRecovery(false)
}

// GetEffectiveCapacitiesWithRecovery 允许调用方传入“连续正常 tick 后已恢复”的判定结果：
// recovered=true 时回到配置容量（不会超过配置），否则按降级口径降至配置以下。
func (c *CentralSchedulerConfig) GetEffectiveCapacitiesWithRecovery(recovered bool) map[CentralResourceKind]int64 {
	out := c.GetCapacities()
	if c == nil || !c.Degraded || recovered {
		return out
	}
	if c.GetDegradedPolicy() == CentralDegradedPolicyBlockNew {
		return map[CentralResourceKind]int64{}
	}
	ratio := c.DegradedRatio
	if ratio <= 0 || ratio > 1 {
		ratio = DefaultCentralDegradedRatio
	}
	for kind, capacity := range out {
		reduced := int64(float64(capacity) * ratio)
		if reduced < 0 {
			reduced = 0
		}
		if reduced == 0 {
			delete(out, kind)
			continue
		}
		out[kind] = reduced
	}
	return out
}

// GetDegradedRecoverTicks 返回自动恢复所需的连续正常 tick 数。
func (c *CentralSchedulerConfig) GetDegradedRecoverTicks() int {
	if c != nil && c.DegradedRecoverTicks > 0 {
		return c.DegradedRecoverTicks
	}
	return DefaultCentralDegradedRecoverTicks
}

// ResourceEnabled 判断某类资源是否已配置且未被禁用（含降级口径）。
func (c *CentralSchedulerConfig) ResourceEnabled(kind CentralResourceKind) bool {
	capacity, ok := c.GetEffectiveCapacities()[kind]
	return ok && capacity > 0
}

// GetDegradedPolicy 返回降级口径，未知值一律按 block_new 处理。
func (c *CentralSchedulerConfig) GetDegradedPolicy() CentralDegradedPolicy {
	if c == nil {
		return CentralDegradedPolicyBlockNew
	}
	switch CentralDegradedPolicy(strings.TrimSpace(strings.ToLower(c.DegradedPolicy))) {
	case CentralDegradedPolicyReduceCapacity:
		return CentralDegradedPolicyReduceCapacity
	default:
		return CentralDegradedPolicyBlockNew
	}
}

// GetAgingWindowTicks 返回防饿死抬升窗口。
func (c *CentralSchedulerConfig) GetAgingWindowTicks() int {
	if c != nil && c.AgingWindowTicks > 0 {
		return c.AgingWindowTicks
	}
	return DefaultCentralAgingWindowTicks
}

// GetAgingStep 返回每个窗口的抬升步长。
func (c *CentralSchedulerConfig) GetAgingStep() int {
	if c != nil && c.AgingStep > 0 {
		return c.AgingStep
	}
	return DefaultCentralAgingStep
}

// GetHighestPriority 返回抬升上限（数值最小）。
func (c *CentralSchedulerConfig) GetHighestPriority() int {
	if c != nil && c.HighestPriority > 0 {
		return c.HighestPriority
	}
	return DefaultCentralHighestPriority
}

// GetDispatchDeadline 返回 pending_confirm 的超时时间。
func (c *CentralSchedulerConfig) GetDispatchDeadline() time.Duration {
	ticks := DefaultCentralDispatchDeadlineTicks
	if c != nil && c.DispatchDeadlineTicks > 0 {
		ticks = c.DispatchDeadlineTicks
	}
	return time.Duration(ticks) * c.GetTickInterval()
}

// GetPendingTTL 返回待处理初始化的保留时长。
func (c *CentralSchedulerConfig) GetPendingTTL() time.Duration {
	seconds := DefaultCentralPendingTTLSecond
	if c != nil && c.PendingTTLSecond > 0 {
		seconds = c.PendingTTLSecond
	}
	return time.Duration(seconds) * time.Second
}

// GetModeSwitchAuthorization 返回某实验的审批后切换授权；不存在返回 nil。
func (c *CentralSchedulerConfig) GetModeSwitchAuthorization(exptID int64) *CentralModeSwitchAuthorization {
	if c == nil || len(c.ModeSwitchAuthorizations) == 0 {
		return nil
	}
	key := strconv.FormatInt(exptID, 10)
	if auth, ok := c.ModeSwitchAuthorizations[key]; ok {
		return &auth
	}
	return nil
}

// GetTargetConcurrency 解析目标并发：实验 ext > 空间级 > 全局。
// ok=false 表示未配置任何目标并发，调用方应沿用 legacy 扫描批量、不做缺口裁剪。
func (c *CentralSchedulerConfig) GetTargetConcurrency(spaceID int64, ext map[string]string) (int, bool) {
	if raw, ok := ext[CentralExtKeyTargetConcurrency]; ok && strings.TrimSpace(raw) != "" {
		if v, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil && v > 0 {
			return v, true
		}
	}
	if c != nil {
		if len(c.SpaceTargetConcurrency) > 0 {
			if v, ok := c.SpaceTargetConcurrency[strconv.FormatInt(spaceID, 10)]; ok && v > 0 {
				return v, true
			}
		}
		if c.DefaultTargetConcurrency > 0 {
			return c.DefaultTargetConcurrency, true
		}
	}
	return 0, false
}

// DefaultCentralResourceCapacitiesList 返回默认资源种类列表的副本。
func DefaultCentralResourceCapacitiesList() []CentralResourceKind {
	out := make([]CentralResourceKind, len(DefaultCentralResourceKinds))
	copy(out, DefaultCentralResourceKinds)
	return out
}

// GetDefaultMode 返回配置的默认模式。
func (c *CentralSchedulerConfig) GetDefaultMode() CentralSchedulerMode {
	if c == nil {
		return CentralSchedulerModeLegacy
	}
	return ParseCentralSchedulerMode(c.DefaultMode)
}

// IsSpaceEnforced 判断空间是否在 enforce 白名单中。
func (c *CentralSchedulerConfig) IsSpaceEnforced(spaceID int64) bool {
	if c == nil {
		return false
	}
	for _, id := range c.EnforceSpaceIDs {
		if id == spaceID {
			return true
		}
	}
	return false
}

// ResolveCentralSchedulerMode 依据实验 ext 与 TCC 配置解析该实验的调度模式。
//
// 优先级：实验 ext 显式值 > 空间白名单 > TCC 默认模式 > legacy。
func ResolveCentralSchedulerMode(cfg *CentralSchedulerConfig, spaceID int64, ext map[string]string) CentralSchedulerMode {
	if raw, ok := ext[CentralExtKeySchedulingMode]; ok && strings.TrimSpace(raw) != "" {
		return ParseCentralSchedulerMode(raw)
	}
	if cfg != nil {
		if cfg.IsSpaceEnforced(spaceID) {
			return CentralSchedulerModeEnforce
		}
		return cfg.GetDefaultMode()
	}
	return CentralSchedulerModeLegacy
}

// ResolveCentralPriority 解析实验优先级（数值越小越优先）。
func ResolveCentralPriority(ext map[string]string, fallback int) int {
	if fallback <= 0 {
		fallback = DefaultCentralPriority
	}
	if raw, ok := ext[CentralExtKeyPriority]; ok {
		return ParseCentralPriority(raw, fallback)
	}
	return fallback
}

// MakeCentralLeaseID 确定性生成 item 级 lease id（兼容无 attempt 的旧调用）。
func MakeCentralLeaseID(exptID, runID, itemID int64) string {
	return MakeCentralLeaseIDWithAttempt(exptID, runID, itemID, 0)
}

// MakeCentralLeaseIDWithAttempt 生成带 attempt 的唯一预占身份。
//
// 每次预占（含重试）都必须使用新的 attempt，避免同一 item 的两次执行复用同一个 lease。
func MakeCentralLeaseIDWithAttempt(exptID, runID, itemID, attempt int64) string {
	return "c:" + strconv.FormatInt(exptID, 10) + ":" + strconv.FormatInt(runID, 10) + ":" +
		strconv.FormatInt(itemID, 10) + ":" + strconv.FormatInt(attempt, 10)
}

// MakeCentralActiveMember 生成活跃集合成员（spaceID:exptID:runID:priority:userID）。
// 带上 spaceID/priority/userID 是为了让中心 tick 无需回查 DB 就能按优先级构造带 Session 的调度事件。
func MakeCentralActiveMember(spaceID, exptID, runID int64, priority int, userID string) string {
	return strconv.FormatInt(spaceID, 10) + ":" + strconv.FormatInt(exptID, 10) + ":" + strconv.FormatInt(runID, 10) +
		":" + strconv.Itoa(priority) + ":" + userID
}

// CentralPriorityScore 把 (effectivePriority, seq) 编码为 zset score：
// 高位为优先级（数值小者优先），低位为单调提交序号，保证同档 FIFO。
func CentralPriorityScore(effectivePriority int, seq int64) float64 {
	seqPart := seq % 1_000_000_000
	if seqPart < 0 {
		seqPart += 1_000_000_000
	}
	return float64(effectivePriority)*1_000_000_000 + float64(seqPart)
}

// EffectiveCentralPriority 计算防饿死抬升后的排序优先级。
//
// windowSeconds 为一个抬升窗口的秒数（= agingWindowTicks × tickInterval）。
// 只影响排序，不修改冻结的 base priority；等待越久提升越多，并在 HighestPriority 封顶。
func EffectiveCentralPriority(basePriority int, registeredAt, now int64, windowSeconds int64, step, highest int) int {
	if basePriority <= 0 {
		basePriority = DefaultCentralPriority
	}
	if windowSeconds <= 0 || step <= 0 || registeredAt <= 0 || now <= registeredAt {
		return basePriority
	}
	windows := int((now - registeredAt) / windowSeconds)
	if windows <= 0 {
		return basePriority
	}
	effective := basePriority - windows*step
	if highest <= 0 {
		highest = DefaultCentralHighestPriority
	}
	if effective < highest {
		effective = highest
	}
	return effective
}

// ParseCentralActiveMember 解析活跃集合成员；非法格式返回 ok=false。
// userID 内可能包含冒号，因此最后一段用 SplitN 保留剩余内容。
func ParseCentralActiveMember(member string) (spaceID, exptID, runID int64, priority int, userID string, ok bool) {
	parts := strings.SplitN(member, ":", 5)
	if len(parts) < 4 {
		return 0, 0, 0, 0, "", false
	}
	var err error
	spaceID, err = strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, 0, 0, 0, "", false
	}
	exptID, err = strconv.ParseInt(parts[1], 10, 64)
	if err != nil || exptID <= 0 {
		return 0, 0, 0, 0, "", false
	}
	runID, err = strconv.ParseInt(parts[2], 10, 64)
	if err != nil || runID <= 0 {
		return 0, 0, 0, 0, "", false
	}
	priority, err = strconv.Atoi(parts[3])
	if err != nil {
		return 0, 0, 0, 0, "", false
	}
	if len(parts) == 5 {
		userID = parts[4]
	}
	return spaceID, exptID, runID, priority, userID, true
}
