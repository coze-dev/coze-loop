// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package benefit

import (
	"context"

	foundationerr "github.com/coze-dev/coze-loop/backend/modules/foundation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

//go:generate mockgen -destination=mocks/benefit_service.go -package=mocks . IBenefitService
type IBenefitService interface {
	// CheckTraceBenefit 校验Trace上报权益
	CheckTraceBenefit(ctx context.Context, param *CheckTraceBenefitParams) (result *CheckTraceBenefitResult, err error)
	// DeductTraceBenefit Trace上报权益扣减
	DeductTraceBenefit(ctx context.Context, param *DeductTraceBenefitParams) (err error)
	// ReplenishExtraTraceBenefit Trace上报权益额外补充
	ReplenishExtraTraceBenefit(ctx context.Context, param *ReplenishExtraTraceBenefitParams) (err error)
	// CheckPromptBenefit 校验Prompt调试权益
	CheckPromptBenefit(ctx context.Context, param *CheckPromptBenefitParams) (result *CheckPromptBenefitResult, err error)
	// CheckEvaluatorBenefit 校验评估器调试权益
	CheckEvaluatorBenefit(ctx context.Context, param *CheckEvaluatorBenefitParams) (result *CheckEvaluatorBenefitResult, err error)
	// CheckAndDeductEvalBenefit 校验扣减评测权益
	CheckAndDeductEvalBenefit(ctx context.Context, param *CheckAndDeductEvalBenefitParams) (result *CheckAndDeductEvalBenefitResult, err error)
	// BatchCheckEnableTypeBenefit 批量校验Enable类型权益
	BatchCheckEnableTypeBenefit(ctx context.Context, param *BatchCheckEnableTypeBenefitParams) (result *BatchCheckEnableTypeBenefitResult, err error)
}

type CheckTraceBenefitParams struct {
	ConnectorUID string `json:"connector_uid"` // Coze登录ID
	SpaceID      int64  `json:"space_id"`      // 空间ID
}

type TraceBenefitCacheStatistics struct {
	CacheVolAcIDMissed               bool `json:"cache_vol_ac_id_missed"`
	CacheCozeBenefitMissed           bool `json:"cache_coze_benefit_missed"`
	CacheCozeLoopExtraBenefitChecked bool `json:"cache_cozeloop_extra_benefit_checked"`
}

type CheckTraceBenefitResult struct {
	AccountAvailable bool  `json:"account_available"`  // 账号是否可用
	IsEnough         bool  `json:"has_reserve"`        // 是否有余量
	StorageDuration  int64 `json:"storage_duration"`   // 存储时长
	WhichIsEnough    int   `json:"which_is_enough"`    // 1走coze，2走cozeloop
	VolcanoAccountID int64 `json:"volcano_account_id"` // 火山账号ID

	CacheStatistics TraceBenefitCacheStatistics `json:"cache_statistics"` // 缓存统计
}

type DeductTraceBenefitParams struct {
	ConnectorUID     string `json:"connector_uid"`      // Coze登录ID
	SpaceID          int64  `json:"space_id"`           // 空间ID
	VolcanoAccountID int64  `json:"volcano_account_id"` // 火山账号ID
	TraceID          string `json:"trace_id"`           // trace id
	Cnt              int64  `json:"cnt"`                // 扣减数量
	Async            *bool  `json:"async,omitempty"`    // 是否异步扣减
	WhichIsEnough    int    `json:"which_is_enough"`    // 1走coze，2走cozeloop
}

type ReplenishExtraTraceBenefitParams struct {
	VolcanoAccountID int64 `json:"volcano_account_id"` // 火山账号ID
	Cnt              int64 `json:"cnt"`                // 补充数量
}

type CheckPromptBenefitParams struct {
	ConnectorUID string `json:"connector_uid"` // Coze登录ID
	SpaceID      int64  `json:"space_id"`      // 空间ID
	PromptID     int64  `json:"prompt_id"`     // prompt id
}

type CheckPromptBenefitResult struct {
	// 拒绝原因，为空代表校验通过
	DenyReason *DenyReason `json:"deny_reason"`
}

type CheckEvaluatorBenefitParams struct {
	ConnectorUID string `json:"connector_uid"` // Coze登录ID
	SpaceID      int64  `json:"space_id"`      // 空间ID
	EvaluatorID  int64  `json:"evaluator_id"`  // prompt id
}

type CheckEvaluatorBenefitResult struct {
	// 拒绝原因，为空代表校验通过，ToErr转化为通用errorx给FE进行通用展示
	DenyReason *DenyReason `json:"deny_reason"`
}

type DenyReason int64

const (
	DenyReasonInsufficient DenyReason = 1 // 余额不足
	DenyReasonExpired      DenyReason = 2 // 订阅到期
	DenyReasonOverdraft    DenyReason = 3 // 账户欠费
)

func (h *DenyReason) ToErr() error {
	switch *h {
	case DenyReasonInsufficient:
		return errorx.NewByCode(foundationerr.AccountInsufficientCodeCode)
	case DenyReasonExpired:
		return errorx.NewByCode(foundationerr.AccountExpiredCodeCode)
	case DenyReasonOverdraft:
		return errorx.NewByCode(foundationerr.AccountOverdraftCodeCode)
	default:
		return nil
	}
}

type CheckAndDeductEvalBenefitParams struct {
	ConnectorUID string            `json:"connector_uid"` // Coze登录ID
	SpaceID      int64             `json:"space_id"`      // 空间ID
	ExperimentID int64             `json:"experiment_id"` // 实验ID
	Ext          map[string]string `json:"ext"`           // extension
}

const (
	ExtKeyExperimentFreeCost = "experiment_free_cost"
)

type CheckAndDeductEvalBenefitResult struct {
	// 拒绝原因，为空代表校验通过，ToErr转化为通用errorx给FE进行通用展示
	DenyReason *DenyReason `json:"deny_reason"`

	// 适用场景：创建实验时校验出是：个人免费版 && 免费次数以内
	// 效果：
	//   传true，后续整个实验不扣次数，不扣资源点
	//   传false，后续整个实验只扣资源点
	// 用法：创建实验时的校验不传，IsFreeEvaluate，如果为true，后续的校验要传
	// 需要改成通过ctx传，Coze还未给出，实验过程中的校验，以及调用prompt/评估器模型等都需要ctx透传给llm gateway
	IsFreeEvaluate *bool `json:"is_free_evaluate"` // 是否特殊检查，免扣权益
}

type BatchCheckEnableTypeBenefitParams struct {
	ConnectorUID       string   `json:"connector_uid"`        // Coze登录ID
	SpaceID            int64    `json:"space_id"`             // 空间ID
	EnableTypeBenefits []string `json:"enable_type_benefits"` // 权益类型列表
}

type BatchCheckEnableTypeBenefitResult struct {
	Results map[string]bool `json:"results"` // 权益类型 -> 是否启用的映射
}
