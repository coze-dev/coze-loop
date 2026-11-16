// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

// SchedulingMode 实验调度模式。创建时冻结；运行中切换必须显式授权。
type SchedulingMode string

const (
	SchedulingModeLegacy        SchedulingMode = "legacy"         // 旧链路：每实验 MQ 自循环 + 每空间 200 运行实验准入
	SchedulingModeCentralShadow SchedulingMode = "central_shadow" // 中心只计算不派发；旧链路继续
	SchedulingModeCentral       SchedulingMode = "central"        // 中心派发；旧链路对该实验静默
)

// 调度元数据在 experiment.eval_conf JSON 扩展中的保留键（不新增表/列）。
const (
	ExtKeySchedulingMode     = "central_scheduling_mode"
	ExtKeySchedulingPriority = "central_priority"
	ExtKeySchedulingEnv      = "central_env"
	ExtKeyWaitSinceMS        = "central_wait_since_ms"
	ExtKeyModeTrace          = "central_mode_trace"
	ExtKeyPriorityTrace      = "central_priority_trace"
	ExtKeySchedulingEpoch    = "central_epoch"
)

const (
	DefaultSchedulingPriority = int32(0)
	MinSchedulingPriority     = int32(0)
	MaxSchedulingPriority     = int32(100)
)

// SchedulingTrace 模式/优先级变更留痕。写入 eval_conf.Ext，按 JSON 持久化。
type SchedulingTrace struct {
	Field         string `json:"field"` // mode / priority
	From          string `json:"from"`
	To            string `json:"to"`
	Operator      string `json:"operator"`
	Reason        string `json:"reason"`
	Authorization string `json:"authorization"`
	AtMS          int64  `json:"at_ms"`
}

// SchedulingMeta 实验调度元数据的领域视图。
type SchedulingMeta struct {
	Mode          SchedulingMode
	Priority      int32
	Env           string
	WaitSinceMS   int64
	Epoch         int64
	ModeTrace     []SchedulingTrace
	PriorityTrace []SchedulingTrace
}

// GetSchedulingMeta 从 eval_conf.Ext 读取调度元数据。缺失 mode 视为 legacy；
// 缺失 env 表示归属不明（中心调度必须 fail-closed，不派发）。
func GetSchedulingMeta(conf *EvaluationConfiguration) SchedulingMeta {
	m := SchedulingMeta{Mode: SchedulingModeLegacy, Priority: DefaultSchedulingPriority}
	if conf == nil || conf.Ext == nil {
		return m
	}
	if v := strings.TrimSpace(conf.Ext[ExtKeySchedulingMode]); v != "" {
		m.Mode = SchedulingMode(v)
	}
	if v := strings.TrimSpace(conf.Ext[ExtKeySchedulingPriority]); v != "" {
		if p, err := strconv.ParseInt(v, 10, 32); err == nil {
			m.Priority = int32(p)
		}
	}
	if v := strings.TrimSpace(conf.Ext[ExtKeySchedulingEnv]); v != "" {
		m.Env = v
	}
	if v := strings.TrimSpace(conf.Ext[ExtKeyWaitSinceMS]); v != "" {
		if w, err := strconv.ParseInt(v, 10, 64); err == nil {
			m.WaitSinceMS = w
		}
	}
	if v := strings.TrimSpace(conf.Ext[ExtKeySchedulingEpoch]); v != "" {
		if e, err := strconv.ParseInt(v, 10, 64); err == nil {
			m.Epoch = e
		}
	}
	m.ModeTrace = decodeSchedulingTraces(conf.Ext[ExtKeyModeTrace])
	m.PriorityTrace = decodeSchedulingTraces(conf.Ext[ExtKeyPriorityTrace])
	return m
}

// SetSchedulingMeta 把调度元数据写回 eval_conf.Ext（调用方必须保留其它键）。
func SetSchedulingMeta(conf *EvaluationConfiguration, meta SchedulingMeta) {
	if conf == nil {
		return
	}
	if conf.Ext == nil {
		conf.Ext = make(map[string]string)
	}
	conf.Ext[ExtKeySchedulingMode] = string(meta.Mode)
	conf.Ext[ExtKeySchedulingPriority] = strconv.FormatInt(int64(meta.Priority), 10)
	if meta.Env != "" {
		conf.Ext[ExtKeySchedulingEnv] = meta.Env
	}
	if meta.WaitSinceMS > 0 {
		conf.Ext[ExtKeyWaitSinceMS] = strconv.FormatInt(meta.WaitSinceMS, 10)
	}
	if meta.Epoch > 0 {
		conf.Ext[ExtKeySchedulingEpoch] = strconv.FormatInt(meta.Epoch, 10)
	}
	if len(meta.ModeTrace) > 0 {
		conf.Ext[ExtKeyModeTrace] = encodeSchedulingTraces(meta.ModeTrace)
	}
	if len(meta.PriorityTrace) > 0 {
		conf.Ext[ExtKeyPriorityTrace] = encodeSchedulingTraces(meta.PriorityTrace)
	}
}

func decodeSchedulingTraces(raw string) []SchedulingTrace {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var traces []SchedulingTrace
	if err := json.Unmarshal([]byte(raw), &traces); err != nil {
		return nil
	}
	return traces
}

func encodeSchedulingTraces(traces []SchedulingTrace) string {
	b, err := json.Marshal(traces)
	if err != nil {
		return ""
	}
	return string(b)
}

// IsCentralDispatchMode 是否由中心真正派发。
func (m SchedulingMode) IsCentralDispatchMode() bool {
	return m == SchedulingModeCentral
}

// IsLegacyLoopMode 是否应继续走旧 MQ 自循环。
func (m SchedulingMode) IsLegacyLoopMode() bool {
	return m == SchedulingModeLegacy || m == SchedulingModeCentralShadow
}

// NormalizeSchedulingPriority 校验并归一化优先级；非法值返回 false。
func NormalizeSchedulingPriority(p int32) (int32, bool) {
	if p < MinSchedulingPriority || p > MaxSchedulingPriority {
		return 0, false
	}
	return p, true
}

// SchedulingApplyInput 创建/运行中修改调度元数据的输入。
type SchedulingApplyInput struct {
	// Authorized 由调用方按 ICentralSchedulingConfigProvider.IsAuthorized 判定；
	// 非 legacy 模式必须为 true，否则拒绝切换。
	Authorized    bool
	Authorization string
	Operator      string
	Reason        string
	Env           string
	NowMS         int64
}

// ApplySchedulingMeta 校验并写入调度元数据。创建时冻结模式；运行中切换必须显式授权。
// 任何跨环境（请求 env 与当前进程 env 不一致）或归属不明的 central 请求都直接拒绝。
func ApplySchedulingMeta(conf *EvaluationConfiguration, in SchedulingApplyInput) error {
	if conf == nil {
		return fmt.Errorf("nil evaluation configuration")
	}
	requested := GetSchedulingMeta(conf)
	if requested.Mode != SchedulingModeLegacy && requested.Mode != SchedulingModeCentral && requested.Mode != SchedulingModeCentralShadow {
		return fmt.Errorf("unknown scheduling mode %q", requested.Mode)
	}
	if requested.Mode != SchedulingModeLegacy && !in.Authorized {
		return fmt.Errorf("scheduling mode %s requires explicit authorization", requested.Mode)
	}
	if strings.TrimSpace(in.Env) == "" {
		return fmt.Errorf("scheduling env is unknown")
	}
	if requested.Env != "" && requested.Env != in.Env {
		return fmt.Errorf("scheduling env %q mismatch current %q", requested.Env, in.Env)
	}
	priority, ok := NormalizeSchedulingPriority(requested.Priority)
	if !ok {
		return fmt.Errorf("scheduling priority %d out of range [%d,%d]", requested.Priority, MinSchedulingPriority, MaxSchedulingPriority)
	}

	now := in.NowMS
	if now <= 0 {
		now = NowUnixMS()
	}
	next := SchedulingMeta{Mode: requested.Mode, Priority: priority, Env: in.Env, WaitSinceMS: requested.WaitSinceMS, Epoch: requested.Epoch + 1}
	next.ModeTrace = requested.ModeTrace
	next.PriorityTrace = requested.PriorityTrace
	if requested.Mode != SchedulingModeLegacy || requested.Env != "" {
		next.ModeTrace = appendSchedulingTrace(next.ModeTrace, SchedulingTrace{
			Field: "mode", From: string(SchedulingModeLegacy), To: string(requested.Mode),
			Operator: in.Operator, Reason: in.Reason, Authorization: in.Authorization, AtMS: now,
		})
	}
	if priority != DefaultSchedulingPriority {
		next.PriorityTrace = appendSchedulingTrace(next.PriorityTrace, SchedulingTrace{
			Field: "priority", From: strconv.FormatInt(int64(DefaultSchedulingPriority), 10), To: strconv.FormatInt(int64(priority), 10),
			Operator: in.Operator, Reason: in.Reason, Authorization: in.Authorization, AtMS: now,
		})
	}
	SetSchedulingMeta(conf, next)
	return nil
}

const maxSchedulingTraces = 50

func appendSchedulingTrace(traces []SchedulingTrace, t SchedulingTrace) []SchedulingTrace {
	traces = append(traces, t)
	if len(traces) > maxSchedulingTraces {
		traces = traces[len(traces)-maxSchedulingTraces:]
	}
	return traces
}

// NowUnixMS 便于测试替换的时间源。
var NowUnixMS = func() int64 { return time.Now().UnixMilli() }

// ResourceDimension 资源维度。
type ResourceDimension string

const (
	ResourceItemExec    ResourceDimension = "item_exec"
	ResourceSandboxExec ResourceDimension = "sandbox_exec"
	ResourceModelCredit ResourceDimension = "model_credit"
)

// AllResourceDimensions 支持的全部维度；可单独停用。
var AllResourceDimensions = []ResourceDimension{
	ResourceItemExec, ResourceSandboxExec, ResourceModelCredit,
}

// ResourceVector 一次预占的资源向量。
type ResourceVector map[ResourceDimension]int64

func (v ResourceVector) Get(dim ResourceDimension) int64 {
	if v == nil {
		return 0
	}
	return v[dim]
}

func (v ResourceVector) Add(other ResourceVector) ResourceVector {
	out := make(ResourceVector, len(v))
	for k, val := range v {
		out[k] = val
	}
	for k, val := range other {
		out[k] += val
	}
	return out
}

func (v ResourceVector) Sub(other ResourceVector) ResourceVector {
	out := make(ResourceVector, len(v))
	for k, val := range v {
		out[k] = val
	}
	for k, val := range other {
		out[k] -= val
		if out[k] < 0 {
			out[k] = 0
		}
	}
	return out
}

// ReservationState 预占状态。RELEASED/EXPIRED 是终态，不可复活。
type ReservationState string

const (
	ReservationStateReserved            ReservationState = "RESERVED"             // 预占中
	ReservationStatePendingCompensation ReservationState = "PENDING_COMPENSATION" // 待补偿（派发结果不明）
	ReservationStateConfirmed           ReservationState = "CONFIRMED"            // 已确认（下游收到）
	ReservationStateReleased            ReservationState = "RELEASED"             // 已释放
	ReservationStateExpired             ReservationState = "EXPIRED"              // 已过期
)

func (s ReservationState) IsTerminal() bool {
	return s == ReservationStateReleased || s == ReservationStateExpired
}

// Reservation 预占记录：额度占用的唯一真值。
type Reservation struct {
	ID          string           `json:"id"`
	Env         string           `json:"env"`
	SpaceID     int64            `json:"space_id"`
	ExptID      int64            `json:"expt_id"`
	RunID       int64            `json:"run_id"`
	ItemID      int64            `json:"item_id"`
	Attempt     int64            `json:"attempt"`
	State       ReservationState `json:"state"`
	Vector      ResourceVector   `json:"vector"`
	Owner       string           `json:"owner"`
	CreatedAt   int64            `json:"created_at"`
	UpdatedAt   int64            `json:"updated_at"`
	ConfirmedAt int64            `json:"confirmed_at,omitempty"`
	ReleasedAt  int64            `json:"released_at,omitempty"`
	Reason      string           `json:"reason,omitempty"`
	Adopted     bool             `json:"adopted,omitempty"`
}

// ActiveExperiment 活跃集成员。
type ActiveExperiment struct {
	ExptID  int64
	RunID   int64
	SpaceID int64
	RunMode ExptRunMode
	// Attempt 本次调度尝试标识；重试会生成新 attempt 并重新预占。
	Attempt     int64
	Priority    int32
	WaitSinceMS int64
	Mode        SchedulingMode
	Env         string
}

// CentralSchedulingConfig 中心调度可配置项；商业版从 TCC/本地配置提供，OSS 默认保守值。
type CentralSchedulingConfig struct {
	Enabled             bool
	TickInterval        time.Duration
	MaxDispatchPerTick  int
	MaxInflightPerExpt  int
	ItemExecCapacity    int64
	SandboxExecCapacity int64
	ModelCreditCapacity int64
	CreditPerItem       int64
	SandboxSlotsPerItem int64
	PublishTimeout      time.Duration
	AckTimeout          time.Duration
	CompensationTimeout time.Duration
	ReconcileInterval   time.Duration
	ReservationTTL      time.Duration
	LeaderLease         time.Duration
	LeaderRenew         time.Duration
	ExptLease           time.Duration
	LegacyAdmissionWait bool
	// DimensionEnabled 为 false 的维度不参与预占与容量校验。
	DimensionEnabled map[ResourceDimension]bool
	// Capacity 为各维度在“同环境全局池”中的总量。
	Capacity map[ResourceDimension]int64
}

// DefaultCentralSchedulingConfig 保守默认：线上默认关闭中心派发；容量/并发均偏小。
func DefaultCentralSchedulingConfig() *CentralSchedulingConfig {
	return &CentralSchedulingConfig{
		Enabled:             false,
		TickInterval:        60 * time.Second,
		MaxDispatchPerTick:  100,
		MaxInflightPerExpt:  5,
		ItemExecCapacity:    100,
		SandboxExecCapacity: 50,
		ModelCreditCapacity: 10000,
		CreditPerItem:       1,
		SandboxSlotsPerItem: 1,
		PublishTimeout:      60 * time.Second,
		AckTimeout:          120 * time.Second,
		CompensationTimeout: 10 * time.Minute,
		ReconcileInterval:   5 * time.Minute,
		ReservationTTL:      72 * time.Hour,
		LeaderLease:         90 * time.Second,
		LeaderRenew:         30 * time.Second,
		ExptLease:           5 * time.Minute,
		DimensionEnabled: map[ResourceDimension]bool{
			ResourceItemExec:    true,
			ResourceSandboxExec: true,
			ResourceModelCredit: true,
		},
		Capacity: map[ResourceDimension]int64{
			ResourceItemExec:    100,
			ResourceSandboxExec: 50,
			ResourceModelCredit: 10000,
		},
	}
}
