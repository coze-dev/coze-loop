// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

// Package central_schedule 定义中心化实验调度的领域模型、端口与纯逻辑。
//
// 该包刻意不依赖 MySQL/Redis/MQ 的具体实现：权威额度账本由 infra 层的
// ReservationLedger 实现（Redis + Lua），本包只定义状态机与调用顺序，便于
// 用 miniredis / fake 做离线验证。
package central_schedule

import (
	"errors"
	"fmt"
	"sort"
	"time"
)

// Mode 实验调度模式。实验创建时固定，重试的新 run 继承实验模式，不可在
// 重试路径上切换。
type Mode int32

const (
	ModeLegacy  Mode = 0
	ModeCentral Mode = 1
)

func (m Mode) Valid() bool { return m == ModeLegacy || m == ModeCentral }

func (m Mode) String() string {
	switch m {
	case ModeLegacy:
		return "legacy"
	case ModeCentral:
		return "central"
	default:
		return fmt.Sprintf("unknown(%d)", int32(m))
	}
}

// ReservationState 预占状态。预占记录本身即账本；MySQL 只做投影。
type ReservationState string

const (
	StateReserved    ReservationState = "reserved"    // 已原子占额度，未发布
	StateDispatching ReservationState = "dispatching" // MQ 已接受，未收到执行确认
	StateConfirmed   ReservationState = "confirmed"   // 执行侧已确认
	StateUncertain   ReservationState = "uncertain"   // 已发布但结果不明
	StateReleased    ReservationState = "released"    // 已归还
	StateExpired     ReservationState = "expired"     // 超期孤儿，已归还
)

func (s ReservationState) Terminal() bool {
	return s == StateReleased || s == StateExpired
}

// Dimension 一个资源维度的额度需求。Key 例：item / evaluator:<id> / sandbox:<tenant>:<type>。
type Dimension struct {
	Key    string `json:"key"`
	Amount int64  `json:"amount"`
}

// Dimensions 对维度做确定性排序并合并同 key，保证 Lua/测试的输入稳定。
func Dimensions(in []Dimension) []Dimension {
	if len(in) == 0 {
		return nil
	}
	merged := make(map[string]int64, len(in))
	for _, d := range in {
		if d.Key == "" || d.Amount < 0 {
			continue
		}
		merged[d.Key] += d.Amount
	}
	out := make([]Dimension, 0, len(merged))
	for k, v := range merged {
		out = append(out, Dimension{Key: k, Amount: v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// Reservation 一次 item attempt 的预占记录（账本权威）。
type Reservation struct {
	ID            string           `json:"id"`
	DedupKey      string           `json:"dedup_key"`
	OwnerScope    string           `json:"owner_scope"`
	ExptID        int64            `json:"expt_id"`
	RunID         int64            `json:"run_id"`
	ItemID        int64            `json:"item_id"`
	Attempt       int32            `json:"attempt"`
	State         ReservationState `json:"state"`
	Dimensions    []Dimension      `json:"dimensions"`
	Generation    int64            `json:"generation"`
	CreatedAt     time.Time        `json:"created_at"`
	UpdatedAt     time.Time        `json:"updated_at"`
	PublishAt     *time.Time       `json:"publish_at,omitempty"`
	AckAt         *time.Time       `json:"ack_at,omitempty"`
	LeaseExpireAt time.Time        `json:"lease_expire_at"`
	Version       int64            `json:"version"`
}

// RunAccount 一个 run 的额度账户。
type RunAccount struct {
	OwnerScope        string           `json:"owner_scope"`
	ExptID            int64            `json:"expt_id"`
	RunID             int64            `json:"run_id"`
	Mode              Mode             `json:"mode"`
	Priority          int32            `json:"priority"`
	TargetConcurrency int64            `json:"target_concurrency"`
	DimensionLimits   map[string]int64 `json:"dimension_limits"`
	Generation        int64            `json:"generation"`
	CreatedAt         time.Time        `json:"created_at"`
}

// RunView 全局扫描返回的活跃 run 视图。
type RunView struct {
	OwnerScope        string
	SpaceID           int64
	ExptID            int64
	RunID             int64
	Mode              Mode
	Priority          int32
	TargetConcurrency int64
	DimensionLimits   map[string]int64
	CreatedAt         time.Time
}

// 已确定的额度维度 key 约定。新增维度只需追加 key，不改账本脚本。
const (
	ItemDimension = "item"
)

func EvaluatorDimension(evaluatorID int64) string {
	return fmt.Sprintf("evaluator:%d", evaluatorID)
}

func SandboxDimension(tenant, resourceType string) string {
	return fmt.Sprintf("sandbox:%s:%s", tenant, resourceType)
}

// ItemStatus item 在 DB 中的执行状态（对账基准）。
type ItemStatus int32

const (
	ItemQueueing   ItemStatus = 0
	ItemProcessing ItemStatus = 1
	ItemSuccess    ItemStatus = 2
	ItemFail       ItemStatus = 3
	ItemTerminal   ItemStatus = 5
)

func (s ItemStatus) Terminal() bool {
	return s == ItemSuccess || s == ItemFail || s == ItemTerminal
}

// ItemView 一次待派发 item。
type ItemView struct {
	SpaceID    int64
	ExptID     int64
	RunID      int64
	ItemID     int64
	Attempt    int32
	Status     ItemStatus
	Dimensions []Dimension
}

// Config 中心调度运行参数（TCC 可覆盖）。
type Config struct {
	// Disabled 为 true 时 tick 直接返回，不扫描、不派发（配置关闭）。
	Disabled          bool
	OwnerScope        string
	TickInterval      time.Duration
	AckTimeout        time.Duration
	CompensationLimit int
	LeaseTTL          time.Duration
	RebuildInterval   time.Duration
	BatchLimit        int
	RunLimit          int
}

func (c Config) WithDefaults() Config {
	if c.TickInterval <= 0 {
		c.TickInterval = time.Minute
	}
	if c.AckTimeout <= 0 {
		c.AckTimeout = 90 * time.Second
	}
	if c.CompensationLimit <= 0 {
		c.CompensationLimit = 3
	}
	if c.LeaseTTL <= 0 {
		c.LeaseTTL = 2 * c.AckTimeout
	}
	if c.RebuildInterval <= 0 {
		c.RebuildInterval = 10 * time.Minute
	}
	if c.BatchLimit <= 0 {
		c.BatchLimit = 200
	}
	if c.RunLimit <= 0 {
		c.RunLimit = 200
	}
	return c
}

// TickStats 单次 tick 的可观测结果。
type TickStats struct {
	// Leader 本实例是否持有 scope lease；false 表示本 tick 被其他实例接管。
	Leader           bool
	ScannedRuns      int
	ScannedItems     int
	Reserved         int
	Dispatched       int
	ReserveShortfall int
	Released         int
	Compensated      int
	Errors           int
}

var (
	ErrOwnerScopeMissing = errors.New("central_schedule: owner scope missing")
	ErrNotInitialized    = errors.New("central_schedule: run ledger not initialized")
	ErrQuotaUnavailable  = errors.New("central_schedule: quota source unavailable")
	ErrReservationExists = errors.New("central_schedule: reservation already exists")
	ErrReservationAbsent = errors.New("central_schedule: reservation absent")
	ErrLateAck           = errors.New("central_schedule: late ack after release")
	ErrStateConflict     = errors.New("central_schedule: reservation state conflict")
)

// ShortfallError 表示某个维度额度不足；整批预占必须零变更。
type ShortfallError struct {
	Dimension string
	Available int64
	Requested int64
}

func (e *ShortfallError) Error() string {
	return fmt.Sprintf("quota shortfall: dimension=%s available=%d requested=%d", e.Dimension, e.Available, e.Requested)
}
