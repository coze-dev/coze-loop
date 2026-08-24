// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package component

import "context"

//go:generate mockgen -destination=mocks/central_reservation_guard.go -package=mocks . ICentralReservationGuard

// ICentralReservationGuard 中心化调度的额度预占校验闸。
//
// OSS 只定义这个最窄的 port，完整的额度账本、调度算法与 Adapter 都在商业版实现并由 Wire 注入。
// 开源部署下注入 noop 实现：legacy 消息完全不受影响，而意外出现的 central 消息 fail-closed
// （拒绝执行），避免无账本环境下绕过额度跑 item。
//
// 为什么 item consumer 必须经过它：中心调度先原子预占额度、再投递 item MQ。若 consumer 不校验
// reservation 就执行，那么迟到的、重复的、账本已重建过的消息都会变成"无额度执行"，全局额度
// 保护即失效。
type ICentralReservationGuard interface {
	// ConfirmRunning 确认该 item 持有额度，取得执行资格。
	//
	// 注意它是**幂等**的而非"一次性"：实现只校验 reservation 存在、并把状态推进到 Running，
	// 对已 Running 的重复调用同样返回 true（见下）。防止同一 item 被并发执行两次**不靠它**，
	// 靠的是 consumer 侧的 item 锁（expt_item_eval_run_lock）。
	//
	// 返回 false 表示 reservation 不存在 —— 调用方**必须放弃执行并丢弃消息**，不得继续跑 item。
	// 已是 Running 的重复投递返回 true：同一 item 的合法原地重试要继续持有原额度，
	// 不重新预占也不重复扣减。
	//
	// schedulerScope 是 Experiment 冻结的调度域，决定去哪本额度账本查这条 reservation。
	// 由调用方从 DB 读出后传入，不由实现方按当前运行环境推断 —— 推断会让"实验属于哪本账"
	// 取决于谁在处理消息，而它本该只取决于数据本身。
	ConfirmRunning(ctx context.Context, schedulerScope string, exptRunID, itemID int64) (bool, error)

	// Release 在 item 进入终态（成功/失败/终止/僵尸清理）时幂等释放额度。
	// 重复调用为 no-op。
	Release(ctx context.Context, schedulerScope string, exptRunID, itemID int64, reason string) error
}

// NewNoopCentralReservationGuard 返回开源部署使用的 noop 实现。
func NewNoopCentralReservationGuard() ICentralReservationGuard {
	return noopCentralReservationGuard{}
}

type noopCentralReservationGuard struct{}

// ConfirmRunning 在无账本环境下一律拒绝。
//
// 选择 fail-closed 而非放行：本方法只会被判定为 enforce 的实验触达，而 enforce 意味着
// "该实验的额度由中心账本管控"。没有账本却放行，等于让实验在无任何额度约束下跑，
// 这比让它停下来等待更危险 —— 后者可见（实验不动会被发现），前者静默（资源被打爆才发现）。
func (noopCentralReservationGuard) ConfirmRunning(ctx context.Context, schedulerScope string, exptRunID, itemID int64) (bool, error) {
	return false, nil
}

// Release 无账本可释放，直接成功返回。
// 不返回错误：终态收口路径不应因为额度模块缺席而失败。
func (noopCentralReservationGuard) Release(ctx context.Context, schedulerScope string, exptRunID, itemID int64, reason string) error {
	return nil
}

//go:generate mockgen -destination=mocks/central_scope_owner.go -package=mocks . ICentralSchedulerScopeOwner

// ICentralSchedulerScopeOwner 判定本进程是否拥有某个调度域。
//
// 与 ICentralReservationGuard 分开是因为职责不同：Guard 回答"这个 item 有没有额度"，
// 本 port 回答"这个 item 该不该由我来跑"。后者是路由/归属问题，即使额度充足也可能
// 必须拒绝执行（消息投错了环境）。
//
// 为什么消息可能投错环境：item MQ 的泳道路由依赖 producer 侧的 x_tt_env tag，
// 而该 tag 会因环境变量缺失、消息重投、broker 配置差异而失效。届时一条 PPE 的 item
// 消息可能被线上 consumer 取到（或反之），若不校验归属就会用一个环境的进程去跑另一个
// 环境的 item，而两个环境共用同一个库 —— 结果直接写进对方的数据。
//
// 开源部署注入 noop（恒定拥有）：单环境部署不存在跨环境投递，无需此校验。
type ICentralSchedulerScopeOwner interface {
	// OwnsSchedulerScope 报告 schedulerScope 是否属于当前进程。
	// 返回 error 表示无法判定（如运行环境探测失败）—— 调用方应重试而非放行。
	OwnsSchedulerScope(ctx context.Context, schedulerScope string) (bool, error)
}

// NewNoopCentralSchedulerScopeOwner 返回开源部署使用的 noop 实现：恒定拥有。
//
// 与 Guard 的 noop 取 fail-closed 相反，本实现取放行。原因是二者防的风险不同：
// 无账本时放行会绕过额度（危险），而单环境部署下不存在"别的环境"，拒绝执行只会让
// 所有 enforce item 永久卡住 —— 那是自造故障，不是防护。
func NewNoopCentralSchedulerScopeOwner() ICentralSchedulerScopeOwner {
	return noopCentralSchedulerScopeOwner{}
}

type noopCentralSchedulerScopeOwner struct{}

func (noopCentralSchedulerScopeOwner) OwnsSchedulerScope(ctx context.Context, schedulerScope string) (bool, error) {
	return true, nil
}

//go:generate mockgen -destination=mocks/central_scope_provider.go -package=mocks . ICentralSchedulerScopeProvider

// ICentralSchedulerScopeProvider 为新建实验解析要冻结的 scheduler_scope。
//
// 与 ICentralSchedulerScopeOwner 的分工：Provider 在**创建期**回答"这个实验该归哪个
// Scope"，Owner 在**执行期**回答"这个 Scope 是不是我的"。分开是因为两者的失败语义相反：
// Provider 解析不出来必须拒绝创建（否则实验冻结了空 Scope，永远不会被任何调度器扫到），
// Owner 判不出来应当重试（可能只是环境探测抖动）。
//
// 开源部署注入 noop：返回空 Scope + nil error，配合 admission 只在 EvalX trigger 下
// 才走 enforce，开源侧不会产生 enforce 实验，因此拿不到 Scope 也无影响。
type ICentralSchedulerScopeProvider interface {
	// ResolveSchedulerScope 返回该空间新建实验应冻结的 Scope。
	// 返回空串且 err 为 nil 表示"本部署不启用中心调度"。
	ResolveSchedulerScope(ctx context.Context, spaceID int64) (string, error)
}

// NewNoopCentralSchedulerScopeProvider 返回开源部署使用的 noop 实现。
func NewNoopCentralSchedulerScopeProvider() ICentralSchedulerScopeProvider {
	return noopCentralSchedulerScopeProvider{}
}

type noopCentralSchedulerScopeProvider struct{}

func (noopCentralSchedulerScopeProvider) ResolveSchedulerScope(ctx context.Context, spaceID int64) (string, error) {
	return "", nil
}

//go:generate mockgen -destination=mocks/central_admission_policy.go -package=mocks . ICentralAdmissionPolicy

// CentralAdmissionSubject 是 admission 判定的输入。
//
// 只带"这个实验是什么"，不带"该不该 enforce" —— 后者是 policy 的职责。
// 字段刻意用基础类型而非 entity.EvalTarget：policy 实现在 commercial，
// 让它依赖 OSS 的领域实体会把整个 target 模型拖进配置层。
type CentralAdmissionSubject struct {
	SpaceID int64
	// TargetType 评测对象类型的配置名（entity.EvalTargetType.ConfigName()，如 "sandbox_agent"）。
	// 空串表示该实验无评测对象（skip-target）或类型未登记配置名。
	TargetType string
	// TargetID 评测对象 ID。0 表示无评测对象。
	TargetID int64
	// QuotaCategories 该实验申报的资源 category 去重列表（已 TrimSpace，顺序不保证）。
	//
	// 为什么 policy 需要它：category 的登记表在 commercial（额度维度是内部资源目录，
	// 不进开源仓），而"申报了一个不存在的 category"必须在**创建期**就拒掉 ——
	// 打错 category 时连类级 wildcard 都兜不住（`sanbox|*` 与 `sandbox|*` 在账本里
	// 毫不相干），该 item 在那一维上完全不受限却真的会去占资源。
	//
	// 只带 category 不带 resource_key：后者的真源是平台侧资源目录（模型/机型清单），
	// 迭代远快于本仓发版，刻意不做名称校验 —— 且它打错时 wildcard 仍然兜得住。
	QuotaCategories []string
}

// CentralAdmissionDecision 是 admission 判定的结果。
//
// 为什么回结构体而不是 bool：缺省优先级也存放在 commercial 的同一份灰度配置里
// （`central_expt_scheduler_space_config.default_priority`），而 OSS 拿不到那份配置。
// 让它随准入结论一起回来，比再加一个 `GetDefaultPriority` 方法更安全 ——
// 两次独立调用之间配置可能热变更，那样同一个实验就可能按"A 配置准入、B 配置定优先级"落库。
type CentralAdmissionDecision struct {
	// Admitted 该实验是否纳入中心调度。
	Admitted bool
	// DefaultPriority 未申报优先级时使用的缺省值（1-99）。
	//
	// 0 表示 policy 未给出意见，调用方按 entity.DefaultExptPriorityLevel（=1）处理。
	// 这条约定让 noop 实现与"TCC 里没配这个字段"两种情况自然收敛到同一行为。
	DefaultPriority int32
}

// ICentralAdmissionPolicy 在创建期收窄中心调度的准入范围。
//
// 它是 trigger 判据之上的**第二道闸**，语义是 AND 而非 OR：
// 只有 trigger 已判定为 EvalX（因而调用方一定申报了 expected_quota_consumption）时
// 才会咨询本 policy，由它决定"这个空间/这类评测对象是否纳入本轮灰度"。
//
// 为什么必须是收窄而不能扩大：enforce 实验强制要求资源消耗向量（缺向量在创建期即报错），
// 而非 EvalX 入口（控制台手动、OpenAPI、定时）目前没有传向量的字段。若 policy 能把
// 这些入口的实验也拽进 enforce，结果是要么创建报错、要么被调度器永远跳过 ——
// 后者表现为"实验建好了但一个 item 都不跑"，且分支静默。等 OpenAPI 具备申报能力后，
// 才可以考虑放宽成 OR。
//
// 开源部署注入 noop（恒定放行）：开源侧不产生 EvalX trigger，本 policy 不会被咨询到。
type ICentralAdmissionPolicy interface {
	// AllowCentralScheduling 报告该实验是否纳入中心调度，并给出缺省优先级。
	//
	// 返回 error 表示配置不可判定。调用方应拒绝创建 enforce 实验而非放行 ——
	// 放行会让一个本该受额度管控的实验绕过管控，且无从发现。
	AllowCentralScheduling(ctx context.Context, subject CentralAdmissionSubject) (CentralAdmissionDecision, error)
}

// NewNoopCentralAdmissionPolicy 返回开源部署使用的 noop 实现：恒定放行。
//
// 取放行而非拒绝：本 policy 的职责是"在已申报向量的实验里再筛一遍"，
// 缺省不筛等于保持 trigger 判据的原有行为，这是引入本 port 前的语义。
func NewNoopCentralAdmissionPolicy() ICentralAdmissionPolicy {
	return noopCentralAdmissionPolicy{}
}

type noopCentralAdmissionPolicy struct{}

// AllowCentralScheduling 放行，且不指定缺省优先级（DefaultPriority=0 → 调用方按 1 处理）。
func (noopCentralAdmissionPolicy) AllowCentralScheduling(ctx context.Context, subject CentralAdmissionSubject) (CentralAdmissionDecision, error) {
	return CentralAdmissionDecision{Admitted: true}, nil
}
