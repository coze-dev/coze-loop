// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package conf

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	loopconf "github.com/coze-dev/coze-loop/backend/pkg/conf"
)

const centralSchedulingKey = "central_scheduling"

// centralSchedulingConf 是配置文件的原始结构；时长统一用秒/毫秒，避免 YAML duration 解析差异。
type centralSchedulingConf struct {
	Enabled *bool `json:"enabled" mapstructure:"enabled"`

	TickIntervalSecond       int64 `json:"tick_interval_second" mapstructure:"tick_interval_second"`
	ReconcileIntervalSecond  int64 `json:"reconcile_interval_second" mapstructure:"reconcile_interval_second"`
	MaxDispatchPerTick       int   `json:"max_dispatch_per_tick" mapstructure:"max_dispatch_per_tick"`
	MaxInflightPerExperiment int   `json:"max_inflight_per_experiment" mapstructure:"max_inflight_per_experiment"`

	CapacityItemExec    *int64 `json:"capacity_item_exec" mapstructure:"capacity_item_exec"`
	CapacitySandboxExec *int64 `json:"capacity_sandbox_exec" mapstructure:"capacity_sandbox_exec"`
	CapacityModelCredit *int64 `json:"capacity_model_credit" mapstructure:"capacity_model_credit"`

	DisableItemExec     bool  `json:"disable_item_exec" mapstructure:"disable_item_exec"`
	DisableSandboxExec  bool  `json:"disable_sandbox_exec" mapstructure:"disable_sandbox_exec"`
	DisableModelCredit  bool  `json:"disable_model_credit" mapstructure:"disable_model_credit"`
	CreditPerItem       int64 `json:"credit_per_item" mapstructure:"credit_per_item"`
	SandboxSlotsPerItem int64 `json:"sandbox_slots_per_item" mapstructure:"sandbox_slots_per_item"`

	PublishTimeoutSecond      int64 `json:"publish_timeout_second" mapstructure:"publish_timeout_second"`
	AckTimeoutSecond          int64 `json:"ack_timeout_second" mapstructure:"ack_timeout_second"`
	CompensationTimeoutSecond int64 `json:"compensation_timeout_second" mapstructure:"compensation_timeout_second"`
	ReservationTTLSecond      int64 `json:"reservation_ttl_second" mapstructure:"reservation_ttl_second"`
	LeaderLeaseSecond         int64 `json:"leader_lease_second" mapstructure:"leader_lease_second"`
	LeaderRenewSecond         int64 `json:"leader_renew_second" mapstructure:"leader_renew_second"`
	ExptLeaseSecond           int64 `json:"expt_lease_second" mapstructure:"expt_lease_second"`
}

type centralSchedulingConfigProvider struct {
	loader loopconf.IConfigLoader
}

func NewCentralSchedulingConfigProvider(factory loopconf.IConfigLoaderFactory) (component.ICentralSchedulingConfigProvider, error) {
	loader, err := factory.NewConfigLoader(consts.EvaluationConfigFileName)
	if err != nil {
		return nil, err
	}
	return &centralSchedulingConfigProvider{loader: loader}, nil
}

func (p *centralSchedulingConfigProvider) raw(ctx context.Context) (*centralSchedulingConf, error) {
	raw := &centralSchedulingConf{}
	// 读配置失败不能静默当成默认值：中心调度按 fail-closed 处理，由上层停止本轮派发。
	if err := p.loader.UnmarshalKey(ctx, centralSchedulingKey, raw); err != nil {
		return nil, fmt.Errorf("load central_scheduling config: %w", err)
	}
	return raw, nil
}

func (p *centralSchedulingConfigProvider) GetCentralSchedulingConfig(ctx context.Context) (*entity.CentralSchedulingConfig, error) {
	raw, err := p.raw(ctx)
	if err != nil {
		return nil, err
	}
	cfg := entity.DefaultCentralSchedulingConfig()
	if raw.Enabled != nil {
		cfg.Enabled = *raw.Enabled
	}
	applySecond := func(v int64, target *time.Duration) {
		if v > 0 {
			*target = time.Duration(v) * time.Second
		}
	}
	applySecond(raw.TickIntervalSecond, &cfg.TickInterval)
	applySecond(raw.ReconcileIntervalSecond, &cfg.ReconcileInterval)
	applySecond(raw.PublishTimeoutSecond, &cfg.PublishTimeout)
	applySecond(raw.AckTimeoutSecond, &cfg.AckTimeout)
	applySecond(raw.CompensationTimeoutSecond, &cfg.CompensationTimeout)
	applySecond(raw.ReservationTTLSecond, &cfg.ReservationTTL)
	applySecond(raw.LeaderLeaseSecond, &cfg.LeaderLease)
	applySecond(raw.LeaderRenewSecond, &cfg.LeaderRenew)
	applySecond(raw.ExptLeaseSecond, &cfg.ExptLease)
	if raw.MaxDispatchPerTick > 0 {
		cfg.MaxDispatchPerTick = raw.MaxDispatchPerTick
	}
	if raw.MaxInflightPerExperiment > 0 {
		cfg.MaxInflightPerExpt = raw.MaxInflightPerExperiment
	}
	setCapacity := func(v *int64, dim entity.ResourceDimension) {
		if v != nil {
			cfg.Capacity[dim] = *v
		}
	}
	setCapacity(raw.CapacityItemExec, entity.ResourceItemExec)
	setCapacity(raw.CapacitySandboxExec, entity.ResourceSandboxExec)
	setCapacity(raw.CapacityModelCredit, entity.ResourceModelCredit)
	if raw.CreditPerItem > 0 {
		cfg.CreditPerItem = raw.CreditPerItem
	}
	if raw.SandboxSlotsPerItem > 0 {
		cfg.SandboxSlotsPerItem = raw.SandboxSlotsPerItem
	}
	cfg.DimensionEnabled = map[entity.ResourceDimension]bool{
		entity.ResourceItemExec:    !raw.DisableItemExec,
		entity.ResourceSandboxExec: !raw.DisableSandboxExec,
		entity.ResourceModelCredit: !raw.DisableModelCredit,
	}
	return cfg, nil
}

func (p *centralSchedulingConfigProvider) GetDimensionCapacity(ctx context.Context, dim entity.ResourceDimension) (bool, int64, error) {
	cfg, err := p.GetCentralSchedulingConfig(ctx)
	if err != nil {
		return false, 0, err
	}
	enabled, ok := cfg.DimensionEnabled[dim]
	if !ok {
		return false, 0, fmt.Errorf("unknown resource dimension %q", dim)
	}
	if !enabled {
		return false, 0, nil
	}
	capacity, ok := cfg.Capacity[dim]
	if !ok || capacity < 0 {
		return false, 0, fmt.Errorf("capacity of dimension %q unknown", dim)
	}
	return true, capacity, nil
}

func (p *centralSchedulingConfigProvider) IsAuthorized(ctx context.Context, operator string, spaceID int64) bool {
	// 授权来源：配置中心白名单；未配置任何白名单时不允许运行期切换到中心调度。
	var raw struct {
		Operators []string `json:"operators" mapstructure:"operators"`
		SpaceIDs  []int64  `json:"space_ids" mapstructure:"space_ids"`
	}
	if err := p.loader.UnmarshalKey(ctx, "central_scheduling_authorization", &raw); err != nil {
		return false
	}
	if operator == "" || spaceID <= 0 {
		return false
	}
	for _, v := range raw.Operators {
		if strings.TrimSpace(v) == operator {
			return true
		}
	}
	for _, v := range raw.SpaceIDs {
		if v == spaceID {
			return true
		}
	}
	return false
}

type environmentProvider struct{}

func NewEnvironmentProvider() component.IEnvironmentProvider { return &environmentProvider{} }

// CurrentEnvID 进程环境即环境归属：PPE 必须带泳道，否则不同泳道会共享同一额度池。
func (p *environmentProvider) CurrentEnvID(ctx context.Context) (string, error) {
	if v := strings.TrimSpace(os.Getenv("COZE_LOOP_CENTRAL_ENV")); v != "" {
		return v, nil
	}
	env := strings.TrimSpace(os.Getenv("SERVICE_ENV"))
	if env == "" {
		env = strings.TrimSpace(os.Getenv("TCE_ENV"))
	}
	if env == "" {
		return "", fmt.Errorf("SERVICE_ENV/TCE_ENV is empty, environment unknown")
	}
	switch strings.ToLower(env) {
	case "ppe", "boe":
		lane := strings.TrimSpace(os.Getenv("RUNTIME_LANE"))
		if lane == "" {
			lane = strings.TrimSpace(os.Getenv("TCE_LANE"))
		}
		if lane == "" {
			return "", fmt.Errorf("%s environment without lane, environment unknown", env)
		}
		return strings.ToLower(env) + ":" + lane, nil
	default:
		return strings.ToLower(env), nil
	}
}
