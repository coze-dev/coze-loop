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

// CentralLease 一次 item 派发的多资源预占凭据。
//
// LeaseID 由 exptID/runID/itemID 确定性生成，因此释放方不需要额外保存映射；
// 重复释放由 Redis 侧 EXISTS 判定为幂等。
type CentralLease struct {
	LeaseID    string                 `json:"lease_id"`
	SpaceID    int64                  `json:"space_id"`
	ExptID     int64                  `json:"expt_id"`
	ExptRunID  int64                  `json:"expt_run_id"`
	ItemID     int64                  `json:"item_id"`
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
	SessionUserID string `json:"session_user_id"`
}

// CentralEnvScope 中心账本的所有权命名空间。
//
// 线上与 PPE 泳道共用 Redis，因此所有 center key 都必须带上 Env + Lane，
// 完整 key 统一以 qn:central: 开头，禁止读写其它前缀。
type CentralEnvScope struct {
	Env  string `json:"env"`
	Lane string `json:"lane"`
}

// Sanitize 将环境/泳道名转换成 key 安全片段：只保留字母数字、下划线、连字符，其余替换为 '_'。
func (s CentralEnvScope) Sanitize() CentralEnvScope {
	env := s.Env
	if strings.TrimSpace(env) == "" {
		env = "default"
	}
	lane := s.Lane
	if strings.TrimSpace(lane) == "" {
		lane = "default"
	}
	return CentralEnvScope{Env: sanitizeKeyPart(env), Lane: sanitizeKeyPart(lane)}
}

func sanitizeKeyPart(in string) string {
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
	if b.Len() == 0 {
		return "default"
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
	// Capacities 资源池容量；缺省使用 DefaultCentralCapacities。
	Capacities map[string]int64 `json:"capacities" mapstructure:"capacities"`
}

// DefaultCentralCapacities 是资源池默认容量。未配置 TCC 时仅用于显式打开中心调度的环境。
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

// GetCapacities 返回有效资源容量，未配置的类别回落到默认值。
func (c *CentralSchedulerConfig) GetCapacities() map[CentralResourceKind]int64 {
	out := make(map[CentralResourceKind]int64, len(DefaultCentralResourceCapacitiesList()))
	for _, kind := range DefaultCentralResourceKinds {
		out[kind] = DefaultCentralCapacities[kind]
	}
	if c == nil {
		return out
	}
	for rawKind, capacity := range c.Capacities {
		if capacity <= 0 {
			continue
		}
		out[CentralResourceKind(rawKind)] = capacity
	}
	return out
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

// MakeCentralLeaseID 确定性生成 item 级 lease id。
func MakeCentralLeaseID(exptID, runID, itemID int64) string {
	return "c:" + strconv.FormatInt(exptID, 10) + ":" + strconv.FormatInt(runID, 10) + ":" + strconv.FormatInt(itemID, 10)
}

// MakeCentralActiveMember 生成活跃集合成员（spaceID:exptID:runID:priority:userID）。
// 带上 spaceID/priority/userID 是为了让中心 tick 无需回查 DB 就能按优先级构造带 Session 的调度事件。
func MakeCentralActiveMember(spaceID, exptID, runID int64, priority int, userID string) string {
	return strconv.FormatInt(spaceID, 10) + ":" + strconv.FormatInt(exptID, 10) + ":" + strconv.FormatInt(runID, 10) +
		":" + strconv.Itoa(priority) + ":" + userID
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
