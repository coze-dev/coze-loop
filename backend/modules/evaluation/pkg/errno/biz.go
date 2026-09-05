package errno

import "fmt"

const (
	mqRetryErrCode = 1

	targetResultErrCode    = 11
	evaluatorResultErrCode = 12
	turnOtherErrCode       = 13

	ServiceInternalErrMsg = "Server internal error"
)

func NeedMQRetry(err error) bool {
	ei, ok := ParseErrImpl(err)
	if ok && ei.Code == mqRetryErrCode {
		return true
	}
	return false
}

func NewMQRetryErr(msg string) error {
	return &ErrImpl{
		Code: mqRetryErrCode,
		Msg:  msg,
	}
}

func WrapMQRetryErr(err error) error {
	return &ErrImpl{
		Code:  mqRetryErrCode,
		Cause: err,
	}
}

func WrapTargetResultErr(err error) error {
	return &ErrImpl{
		Code:  targetResultErrCode,
		Cause: err,
	}
}

func NewTargetResultErr(msg string) error {
	return &ErrImpl{
		Code: targetResultErrCode,
		Msg:  msg,
	}
}

func ParseTargetResultErr(err error) (bool, string) {
	ei, ok := ParseErrImpl(err)
	if ok && ei.Code == targetResultErrCode {
		return true, ei.ErrMsg()
	}
	return false, ""
}

func WrapEvaluatorResultErr(err error) error {
	return &ErrImpl{
		Code:  evaluatorResultErrCode,
		Cause: err,
	}
}

func NewEvaluatorResultErr(msg string) error {
	return &ErrImpl{
		Code: evaluatorResultErrCode,
		Msg:  msg,
	}
}

func ParseEvaluatorResultErr(err error) (bool, string) {
	ei, ok := ParseErrImpl(err)
	if ok && ei.Code == evaluatorResultErrCode {
		return true, ei.ErrMsg()
	}
	return false, ""
}

func WrapTurnOtherErr(err error) error {
	return &ErrImpl{
		Code:  turnOtherErrCode,
		Cause: err,
	}
}

func NewTurnOtherErr(msg string, err error) error {
	return &ErrImpl{
		Code:  turnOtherErrCode,
		Msg:   msg,
		Cause: err,
	}
}

func ParseTurnOtherErr(err error) (bool, string) {
	ei, ok := ParseErrImpl(err)
	if ok && ei.Code == turnOtherErrCode {
		return true, ei.ErrMsg()
	}
	return false, ""
}

// NewExptZombieTimeoutErr 构造"实验整体超时"错误。zombieSecond 为触发阈值，用于给用户看清楚是多久没有更新触发的。
func NewExptZombieTimeoutErr(zombieSecond int64, exptID, exptRunID int64) error {
	msg := fmt.Sprintf("实验已超过最大执行时长 %ds 被系统超时终止 (expt_id=%d, expt_run_id=%d)", zombieSecond, exptID, exptRunID)
	return &ErrImpl{
		Code: ExptZombieTimeoutCode,
		Msg:  msg,
	}
}

// NewItemZombieTimeoutErr 构造"实验行僵尸超时"错误。zombieSecond 为触发阈值，asyncExec 用于区分同步/异步阈值。
func NewItemZombieTimeoutErr(zombieSecond int, asyncExec bool) error {
	mode := "同步"
	if asyncExec {
		mode = "异步"
	}
	msg := fmt.Sprintf("实验行长时间未更新 (超过%s模式阈值 %ds)，已被系统判定为僵尸并置为失败", mode, zombieSecond)
	return &ErrImpl{
		Code: ItemZombieTimeoutCode,
		Msg:  msg,
	}
}

// ParseItemZombieTimeoutErr 反解 item 超时错误，返回 (是否命中, 用户可见的详细描述)。
func ParseItemZombieTimeoutErr(err error) (bool, string) {
	ei, ok := ParseErrImpl(err)
	if ok && ei.Code == ItemZombieTimeoutCode {
		return true, ei.ErrMsg()
	}
	return false, ""
}

// NewItemQuotaImpossibleErr 构造"申报量超过调度域上限、任何配置下都无法调度"错误。
//
// 与僵尸超时的本质区别：僵尸是"卡了太久"（时间维度），它是"结构上就放不下"（配置维度）——
// 等多久都不会变化，所以调度器在授予点直接置失败而不是继续排队。key/amount/limit 一并写进
// err_msg，让用户据此改小申报或调大上限后重建实验。
func NewItemQuotaImpossibleErr(constraintKey string, amount, limit int64) error {
	msg := fmt.Sprintf("实验行申报的 %s 资源量（%d）超过调度域上限（%d），任何配置下都无法调度，请改小申报或调大上限后重建实验",
		constraintKey, amount, limit)
	return &ErrImpl{
		Code: ItemQuotaImpossibleCode,
		Msg:  msg,
	}
}

// ParseItemQuotaImpossibleErr 反解额度不可满足错误，返回 (是否命中, 用户可见的详细描述)。
func ParseItemQuotaImpossibleErr(err error) (bool, string) {
	ei, ok := ParseErrImpl(err)
	if ok && ei.Code == ItemQuotaImpossibleCode {
		return true, ei.ErrMsg()
	}
	return false, ""
}

// NewSandboxTerminatedBeforeReportErr 构造 "沙箱提前终态导致行失败" 错误。sandboxStatus 传沙箱返回的状态字面量（如 "Failed"/"Canceled"），
// 用于把根因附在 err_msg 上供 API 展示。触发场景见 ExptSchedulerImpl.sweepTerminatedSandboxItems。
func NewSandboxTerminatedBeforeReportErr(sandboxStatus string) error {
	msg := fmt.Sprintf("沙箱在结果上报前已进入终态 (%s)，该实验行已置为失败", sandboxStatus)
	return &ErrImpl{
		Code: SandboxTerminatedBeforeReportCode,
		Msg:  msg,
	}
}
