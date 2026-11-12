// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"fmt"
	"strings"
	"time"
)

// SchedulingMode 实验调度方式。创建时冻结，运行期不可修改。
// 缺省/空值一律按 legacy 处理，保证旧实验与旧调用方行为不变。
type SchedulingMode string

const (
	SchedulingModeLegacy  SchedulingMode = "legacy"
	SchedulingModeCentral SchedulingMode = "central"
)

// NormalizeSchedulingMode 把外部传入的字符串收敛为已知枚举。
// 未知值不回落到 central，避免旧调用方因为拼写错误被意外切到新链路。
func NormalizeSchedulingMode(raw string) SchedulingMode {
	switch SchedulingMode(strings.ToLower(strings.TrimSpace(raw))) {
	case SchedulingModeCentral:
		return SchedulingModeCentral
	case SchedulingModeLegacy:
		return SchedulingModeLegacy
	default:
		return SchedulingModeLegacy
	}
}

func (m SchedulingMode) Valid() bool {
	return m == SchedulingModeLegacy || m == SchedulingModeCentral
}

// SchedulingConfig 冻结在 experiment.eval_conf 中的调度元数据。
// 不新增 experiment 表列，避免线上/PPE 共用 MySQL 时的 DDL 成本。
type SchedulingConfig struct {
	Mode SchedulingMode `json:"mode,omitempty" mapstructure:"mode"`
	// OwnerEnv 实验归属环境。prod=线上；ppe_xxx=PPE 泳道。
	// 空值表示归属不清，中心调度不得调度该实验。
	OwnerEnv string `json:"owner_env,omitempty" mapstructure:"owner_env"`
	// Priority 越大越先调度；缺省 0。
	Priority int32 `json:"priority,omitempty" mapstructure:"priority"`
	// FrozenAt 创建时冻结时间（Unix 秒）。
	FrozenAt int64 `json:"frozen_at,omitempty" mapstructure:"frozen_at"`
}

func (c *SchedulingConfig) EffectiveMode() SchedulingMode {
	if c == nil {
		return SchedulingModeLegacy
	}
	if c.Mode == SchedulingModeCentral {
		return SchedulingModeCentral
	}
	return SchedulingModeLegacy
}

func (c *SchedulingConfig) IsCentral() bool {
	return c != nil && c.Mode == SchedulingModeCentral
}

// SchedulingOwnerEnvExtKey 内部扩展键：commercial 接入层把当前服务实例的 env 冻结进
// experiment.eval_conf.scheduling.owner_env，开源 OSS 侧不直接依赖平台 env 包。
const SchedulingOwnerEnvExtKey = "__cs_owner_env"

// BuildSchedulingConfig 由外部可选字段构建冻结配置。空 mode 且 priority=0 时返回 nil，
// 让旧实验继续走“无 scheduling 子对象 = legacy”的兼容路径。
func BuildSchedulingConfig(modeRaw string, priority int32, ext map[string]string) *SchedulingConfig {
	mode := NormalizeSchedulingMode(modeRaw)
	if mode == SchedulingModeLegacy && priority == 0 {
		return nil
	}
	cfg := &SchedulingConfig{
		Mode:     mode,
		Priority: priority,
		FrozenAt: time.Now().Unix(),
	}
	if mode == SchedulingModeCentral {
		cfg.OwnerEnv = NormalizeOwnerEnv(ext[SchedulingOwnerEnvExtKey])
	}
	return cfg
}

// NormalizeOwnerEnv 规范化运行环境标识：
//   - 空值保持空，表示归属不清；
//   - prod/online 统一为 prod；
//   - 其余（如 ppe_xxx、boe_xxx）原样保留小写。
func NormalizeOwnerEnv(raw string) string {
	env := strings.ToLower(strings.TrimSpace(raw))
	switch env {
	case "":
		return ""
	case "prod", "online", "product":
		return "prod"
	default:
		return env
	}
}

func (c *SchedulingConfig) ValidateForCentral() error {
	if c == nil || c.Mode != SchedulingModeCentral {
		return fmt.Errorf("scheduling mode is not central")
	}
	if strings.TrimSpace(c.OwnerEnv) == "" {
		return fmt.Errorf("owner_env is required for central scheduling")
	}
	return nil
}

// 资源额度维度。首期只启用两维；ledger 以 map/键列表承载，后续可加维度。
const (
	ResourceItemSlots      = "item_slots"
	ResourceEvaluatorSlots = "evaluator_slots"
)

// ResourceVector 一次预占/一份容量的资源向量。
type ResourceVector struct {
	ItemSlots      int64 `json:"item_slots" mapstructure:"item_slots"`
	EvaluatorSlots int64 `json:"evaluator_slots" mapstructure:"evaluator_slots"`
}

func (v ResourceVector) Add(other ResourceVector) ResourceVector {
	return ResourceVector{
		ItemSlots:      v.ItemSlots + other.ItemSlots,
		EvaluatorSlots: v.EvaluatorSlots + other.EvaluatorSlots,
	}
}

func (v ResourceVector) Sub(other ResourceVector) ResourceVector {
	return ResourceVector{
		ItemSlots:      v.ItemSlots - other.ItemSlots,
		EvaluatorSlots: v.EvaluatorSlots - other.EvaluatorSlots,
	}
}

func (v ResourceVector) IsZero() bool {
	return v.ItemSlots == 0 && v.EvaluatorSlots == 0
}

func (v ResourceVector) Validate() error {
	if v.ItemSlots < 0 || v.EvaluatorSlots < 0 {
		return fmt.Errorf("resource vector must not be negative: %+v", v)
	}
	return nil
}

// Covers 判断 v 是否覆盖 need（各维度均 >=）。
func (v ResourceVector) Covers(need ResourceVector) bool {
	return v.ItemSlots >= need.ItemSlots && v.EvaluatorSlots >= need.EvaluatorSlots
}

// ReservationState 预占记录的生命周期。
type ReservationState string

const (
	ReservationStateReserved    ReservationState = "reserved"
	ReservationStateDispatching ReservationState = "dispatching"
	ReservationStateDispatched  ReservationState = "dispatched"
	ReservationStateUnknown     ReservationState = "unknown"
	ReservationStateReleased    ReservationState = "released"
)

// Reservation 一个 item 的额度预占。ID 对 (run,item) 确定，重试复用同一 key。
type Reservation struct {
	ID         string
	SpaceID    int64
	ExptID     int64
	ExptRunID  int64
	ItemID     int64
	OwnerEnv   string
	Vector     ResourceVector
	DispatchID string
	State      ReservationState
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// ReservationID 生成确定性的预占标识；env 由 Redis key 前缀隔离，不进入 ID。
func ReservationID(exptRunID, itemID int64) string {
	return fmt.Sprintf("%d:%d", exptRunID, itemID)
}

// CentralActiveExperiment 中心调度扫描出的活跃实验视图。
type CentralActiveExperiment struct {
	SpaceID            int64
	ExptID             int64
	ExptRunID          int64
	ExptRunMode        ExptRunMode
	ExptType           ExptType
	ExptStatus         ExptStatus
	ItemConcurNum      int
	EvaluatorConcurNum int
	Priority           int32
	OwnerEnv           string
	CreatedBy          string
	// RetryItemIDs 仅 RetryItems 模式非空，来自 expt_run_log.item_ids；中心 tick 需要按
	// 原重试目标执行，不能退化成全量扫描。
	RetryItemIDs []int64
	StartAt      time.Time
	UpdatedAt    time.Time
}

// Gap 返回还能补多少 item；<=0 表示当前没有并发缺口。
func (a *CentralActiveExperiment) Gap(processing int) int {
	if a == nil {
		return 0
	}
	gap := a.ItemConcurNum - processing
	if gap < 0 {
		return 0
	}
	return gap
}

// ResourcePerItem 单个 item 需要预占的额度向量。
func (a *CentralActiveExperiment) ResourcePerItem() ResourceVector {
	evaluator := int64(a.EvaluatorConcurNum)
	if evaluator <= 0 {
		evaluator = 3 // 与 EvaluatorsConf.GetEvaluatorConcurNum 的默认值一致
	}
	return ResourceVector{ItemSlots: 1, EvaluatorSlots: evaluator}
}

// CentralItemState 中心调度结算所需的 item 实际状态（权威源为 DB）。
type CentralItemState struct {
	ItemID    int64
	Status    ItemRunState
	UpdatedAt time.Time
}

// CentralSchedulerConf 中心调度运行配置，来自 TCC。
type CentralSchedulerConf struct {
	Enabled                 bool                      `json:"enabled" mapstructure:"enabled"`
	TickIntervalSeconds     int                       `json:"tick_interval_seconds" mapstructure:"tick_interval_seconds"`
	LeaseTTLSeconds         int                       `json:"lease_ttl_seconds" mapstructure:"lease_ttl_seconds"`
	LeaseRenewSeconds       int                       `json:"lease_renew_seconds" mapstructure:"lease_renew_seconds"`
	MaxBatchPerExpt         int                       `json:"max_batch_per_expt" mapstructure:"max_batch_per_expt"`
	AgingPointsPerMinute    int                       `json:"aging_points_per_minute" mapstructure:"aging_points_per_minute"`
	SettleWindowSeconds     int                       `json:"settle_window_seconds" mapstructure:"settle_window_seconds"`
	ReconcileIntervalSecond int                       `json:"reconcile_interval_seconds" mapstructure:"reconcile_interval_seconds"`
	DriftItemThreshold      int64                     `json:"drift_item_threshold" mapstructure:"drift_item_threshold"`
	DriftEvaluatorThreshold int64                     `json:"drift_evaluator_threshold" mapstructure:"drift_evaluator_threshold"`
	DefaultCapacity         ResourceVector            `json:"default" mapstructure:"default"`
	EnvOverrides            map[string]ResourceVector `json:"env_overrides" mapstructure:"env_overrides"`
}

func DefaultCentralSchedulerConf() *CentralSchedulerConf {
	return &CentralSchedulerConf{
		Enabled:                 false,
		TickIntervalSeconds:     5,
		LeaseTTLSeconds:         15,
		LeaseRenewSeconds:       5,
		MaxBatchPerExpt:         50,
		AgingPointsPerMinute:    0,
		SettleWindowSeconds:     40,
		ReconcileIntervalSecond: 60,
		DriftItemThreshold:      5,
		DriftEvaluatorThreshold: 15,
		DefaultCapacity:         ResourceVector{ItemSlots: 2000, EvaluatorSlots: 6000},
	}
}

func (c *CentralSchedulerConf) FillDefaults() {
	if c == nil {
		return
	}
	d := DefaultCentralSchedulerConf()
	if c.TickIntervalSeconds <= 0 {
		c.TickIntervalSeconds = d.TickIntervalSeconds
	}
	if c.LeaseTTLSeconds <= 0 {
		c.LeaseTTLSeconds = d.LeaseTTLSeconds
	}
	if c.LeaseRenewSeconds <= 0 {
		c.LeaseRenewSeconds = d.LeaseRenewSeconds
	}
	if c.MaxBatchPerExpt <= 0 {
		c.MaxBatchPerExpt = d.MaxBatchPerExpt
	}
	if c.SettleWindowSeconds <= 0 {
		c.SettleWindowSeconds = d.SettleWindowSeconds
	}
	if c.ReconcileIntervalSecond <= 0 {
		c.ReconcileIntervalSecond = d.ReconcileIntervalSecond
	}
	if c.DriftItemThreshold <= 0 {
		c.DriftItemThreshold = d.DriftItemThreshold
	}
	if c.DriftEvaluatorThreshold <= 0 {
		c.DriftEvaluatorThreshold = d.DriftEvaluatorThreshold
	}
	if c.DefaultCapacity.IsZero() {
		c.DefaultCapacity = d.DefaultCapacity
	}
}

// Capacity 返回当前 env 的容量配置；env override 优先。
func (c *CentralSchedulerConf) Capacity(ownerEnv string) ResourceVector {
	if c == nil {
		return ResourceVector{}
	}
	if c.EnvOverrides != nil {
		if v, ok := c.EnvOverrides[ownerEnv]; ok {
			return v
		}
	}
	return c.DefaultCapacity
}

// TickInterval 返回 tick 间隔。
func (c *CentralSchedulerConf) TickInterval() time.Duration {
	if c == nil || c.TickIntervalSeconds <= 0 {
		return 5 * time.Second
	}
	return time.Duration(c.TickIntervalSeconds) * time.Second
}

func (c *CentralSchedulerConf) LeaseTTL() time.Duration {
	if c == nil || c.LeaseTTLSeconds <= 0 {
		return 15 * time.Second
	}
	return time.Duration(c.LeaseTTLSeconds) * time.Second
}

func (c *CentralSchedulerConf) LeaseRenew() time.Duration {
	if c == nil || c.LeaseRenewSeconds <= 0 {
		return 5 * time.Second
	}
	return time.Duration(c.LeaseRenewSeconds) * time.Second
}

func (c *CentralSchedulerConf) SettleWindow() time.Duration {
	if c == nil || c.SettleWindowSeconds <= 0 {
		return 40 * time.Second
	}
	return time.Duration(c.SettleWindowSeconds) * time.Second
}
