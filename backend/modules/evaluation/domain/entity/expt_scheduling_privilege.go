// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"strconv"
	"strings"
)

// ExptSchedulingPrivilegeWhiteList 决定"谁可以申报中心调度的特权参数"。
//
// 它管**三样**自报参数，判据统一为同一份名单：
//
//	priority_level               —— 调度优先级（参与严格优先级排序，高优持续抢占额度）
//	expected_quota_consumption   —— 单 item 预期资源消耗（决定扣多少额度）
//	trigger_type = evalx         —— 能否让实验进入 enforce（被中心调度纳管）
//
// 为什么必须收在一处：这三样都是**调用方在请求体里自报的**，而它们共同决定"这个实验
// 拿多少资源、排在谁前面"。三样各配一套判据只会让"谁有特权"散成三处、缺省方向还可能
// 不一致；收成一份名单，答案只有一个地方。
//
// 各自不设限的后果：
//   - priority → 一个人把自己所有实验设成 99 就能让别人的实验饿死
//   - quota    → 虚报消耗（少报偷跑、多报占死额度）
//   - trigger  → 任何人自称 evalx 就能进 enforce，"谁被纳管"不再由我们决定
//
// 三样都不违反任何校验、不报错，所以都必须靠名单挡。
//
// ★ 三个身份维度，全部由**我们**维护，不接受用户自助配置：
//
//	UserEmails —— 点名的自然人（邮箱，人可读）
//	SpaceIDs   —— 点名的空间
//	CallerPSMs —— 点名的可信服务（EvalX 等系统调用方走这一维）
//
// ⚠️ SpaceIDs 的正确用法是「**只有管理员在的私有空间**」：给那样的空间开白名单，
// 等价于给一份受控的人员名单开白名单。**绝不要**把普通业务空间填进来 ——
// 业务空间里谁都能建实验、成员还会随时增减，那等于把特权下放给一群不确定的人。
//
// 与 enforce 灰度（`central_expt_scheduler_space_config`）的性质区别值得记住：
// 那份按空间/评测对象划范围是**运维范围**（谁被中心调度纳管）；本表是**特权授予**
// （谁能自报参数）。前者配错只是纳管范围不对，后者配错是有人能插队/虚报资源。
//
// 三个维度之间是 **OR**：命中任意一个即放行。不能取 AND —— CallerPSMs 服务的是系统调用方，
// 它没有自然人 user，取 AND 会让这一维永远走不通。
type ExptSchedulingPrivilegeWhiteList struct {
	// UserEmails 可申报特权参数的用户邮箱。
	//
	// 用邮箱而不是 user_id：这份名单由人手工维护、也要靠人 review，
	// `zhangsan@bytedance.com` 一眼就知道是谁，而 `7123456789012345678` 需要另查一次才能确认
	// —— 加错人是"给了特权"，最不该靠肉眼比对长数字来防。
	//
	// 邮箱取自**已验证的 ByteTIM ticket claim**（见商业版 infra/middleware/user.go），
	// 不是请求体里的字段，调用方无法伪造，因此可以当授权键用。
	UserEmails []string `json:"user_emails" mapstructure:"user_emails"`
	// SpaceIDs 整个空间放行。**只填管理员私有空间**，理由见类型注释。
	//
	// 用 []string 而非 []int64：19 位雪花 ID 超出 float64 安全整数范围，
	// 任何把 JSON number 当 double 的环节都会静默截断低位（实测 bytedcli tcc 写入
	// 7533128632407949313 会回读成 ...949000）。配置里一律写 ["7533..."]。
	SpaceIDs []string `json:"space_ids" mapstructure:"space_ids"`
	// CallerPSMs 可信调用方 PSM。EvalX 是 `stone.cozeloop.evalx`。
	//
	// ⚠️ 只能填**内部 RPC 直连**的 PSM。它取自 kitex 的 caller 字段，由框架按调用方身份填充，
	// 调用方无法在业务参数里伪造；这与 trigger_type 有本质区别 —— 后者是请求体里的普通字段，
	// 任何人都能自称 "evalx"，因此**绝不能**反过来用 trigger_type 当授权判据。
	//
	// ⚠️ **上线检查项**：EvalX 的 PSM 必须在部署前填进这里。缺了它，全部 EvalX 实验会
	// 静默退回 legacy —— 中心调度一个候选都没有，现象是"实验都在跑但一个都不受额度管控"，
	// 而这个方向的失败是无声的。
	CallerPSMs []string `json:"caller_psms" mapstructure:"caller_psms"`
	// AllowAll 全部放行。仅用于"暂时不限制"的过渡期，不建议长期开启。
	AllowAll bool `json:"allow_all" mapstructure:"allow_all"`
}

// DefaultExptSchedulingPrivilegeWhiteList 配置缺失或解析失败时的兜底：**谁都没有特权**。
//
// 取禁止而非放行：这三样参数被滥用的后果都是"悄悄多占资源/插队"。配置中心抖动时宁可让
// 大家退回缺省行为（priority=1、无向量、legacy 链路 —— 都可见且无损），也不要因为
// 读不到配置就放开特权（静默、且要等资源被抢占才发现）。
func DefaultExptSchedulingPrivilegeWhiteList() *ExptSchedulingPrivilegeWhiteList {
	return &ExptSchedulingPrivilegeWhiteList{}
}

// ExptSchedulingPrivilegeSubject 是判定的输入：一次创建实验请求里与"能否申报特权"有关的全部身份。
//
// 收成一个结构体而不是散着传：三者都是可选的（自然人调用时 CallerPSM 为空，
// 系统调用时 UserEmail 为空），散着传容易在新增维度时漏掉调用点。
type ExptSchedulingPrivilegeSubject struct {
	// UserEmail 已验证的用户邮箱（来自 session）。空串表示拿不到身份。
	UserEmail string
	SpaceID   int64
	CallerPSM string
}

// AllowSchedulingPrivilege 报告该请求是否可以申报中心调度的特权参数
// （priority / quota 向量 / evalx trigger 三者同一判据）。
//
// nil 白名单返回 false（与本仓其它白名单一致：没配置就是没开）。
func (w *ExptSchedulingPrivilegeWhiteList) AllowSchedulingPrivilege(subject ExptSchedulingPrivilegeSubject) bool {
	if w == nil {
		return false
	}
	if w.AllowAll {
		return true
	}
	if w.matchUserEmail(subject.UserEmail) {
		return true
	}
	if w.matchSpaceID(subject.SpaceID) {
		return true
	}
	return w.matchCallerPSM(subject.CallerPSM)
}

// matchSpaceID 把入参格式化成字符串再比。
//
// spaceID=0 表示"无空间上下文"，一律不匹配 —— 否则运维在 space_ids 里误填 "0"
// 就会把所有无空间上下文的请求放行。
func (w *ExptSchedulingPrivilegeWhiteList) matchSpaceID(spaceID int64) bool {
	if spaceID == 0 {
		return false
	}
	want := strconv.FormatInt(spaceID, 10)
	for _, id := range w.SpaceIDs {
		if strings.TrimSpace(id) == want {
			return true
		}
	}
	return false
}

// matchUserEmail 忽略大小写与首尾空白比对（邮箱本身大小写不敏感，且名单由人手写）。
//
// 空邮箱一律不匹配：拿不到身份时不得放行 —— 不把"读不到"当成有权限。
// 注意商业版只在 ByteTIM ticket 存在时才填 Email，所以非用户态调用（纯服务间 RPC）
// 这里必然为空，那种情况该走 CallerPSMs 维度。
func (w *ExptSchedulingPrivilegeWhiteList) matchUserEmail(userEmail string) bool {
	email := strings.TrimSpace(userEmail)
	if email == "" {
		return false
	}
	for _, e := range w.UserEmails {
		if strings.EqualFold(strings.TrimSpace(e), email) {
			return true
		}
	}
	return false
}

// matchCallerPSM 忽略大小写与首尾空白比对。
//
// 容忍这两者是因为 PSM 由人手写进 TCC，"Stone.CozeLoop.Foo " 这类笔误的后果是静默不放行
// —— 配了却不生效，而且两个字符串肉眼几乎一样，极难反推。
// 空 caller 一律不匹配：非 RPC 直连（HTTP 入口）时 caller 为空，那种情况该走 email/space 维度。
func (w *ExptSchedulingPrivilegeWhiteList) matchCallerPSM(callerPSM string) bool {
	caller := strings.TrimSpace(callerPSM)
	if caller == "" {
		return false
	}
	for _, psm := range w.CallerPSMs {
		if strings.EqualFold(strings.TrimSpace(psm), caller) {
			return true
		}
	}
	return false
}
