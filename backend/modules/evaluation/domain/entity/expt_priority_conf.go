// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// ExptPriorityWhiteList 决定"谁可以在发起实验时指定调度优先级"。
//
// 为什么需要这道闸：priority 在中心调度下参与**严格优先级排序**，高优实验会持续抢占额度。
// 字段本身在 IDL 里带 json/form 绑定标签，任何有建实验权限的调用方都能填 —— 不加限制的话
// 一个人把自己所有实验设成 99 就能让别人的实验饿死，而这既不违反任何校验也不会报错。
//
// ★ 三个维度，全部由**我们**维护，不接受用户自助配置：
//
//	UserEmails —— 点名的自然人（邮箱，人可读）
//	SpaceIDs   —— 点名的空间
//	CallerPSMs —— 点名的可信服务
//
// ⚠️ SpaceIDs 的正确用法是「**只有管理员在的私有空间**」：给那样的空间开白名单，
// 等价于给一份受控的人员名单开白名单。**绝不要**把普通业务空间填进来 ——
// 业务空间里谁都能建实验、成员还会随时增减，那等于把插队权下放给一群不确定的人，
// 而这份特权名单必须始终由我们掌握。
//
// 与 enforce 灰度（`central_expt_scheduler_space_config`）的性质区别值得记住：
// 那份按空间/评测对象划范围是**运维范围**（谁被中心调度纳管）；本表是**特权授予**
// （谁能抢资源）。前者配错只是纳管范围不对，后者配错是有人能插队。
//
// 三个维度之间是 **OR**：命中任意一个即放行。不能取 AND —— CallerPSMs 服务的是系统调用方，
// 它没有自然人 user，取 AND 会让这一维永远走不通。
type ExptPriorityWhiteList struct {
	// UserEmails 可指定 priority 的用户邮箱。
	//
	// 用邮箱而不是 user_id：这份名单由人手工维护、也要靠人 review，
	// `zhangsan@bytedance.com` 一眼就知道是谁，而 `7123456789012345678` 需要另查一次才能确认
	// —— 加错人是"给了插队权"，是最不该靠肉眼比对长数字来防的错误。
	//
	// 邮箱取自**已验证的 ByteTIM ticket claim**（见商业版 infra/middleware/user.go），
	// 不是请求体里的字段，调用方无法伪造，因此可以当授权键用。
	UserEmails []string `json:"user_emails" mapstructure:"user_emails"`
	// SpaceIDs 整个空间放行。**只填管理员私有空间**，理由见类型注释。
	// 类型见 ConfigIDList：数字与字符串两种写法都接受（19 位雪花 ID 建议写字符串）。
	SpaceIDs ConfigIDList `json:"space_ids" mapstructure:"space_ids"`
	// CallerPSMs 可信调用方 PSM（如 EvalX 的服务名）。用于系统调用方 —— 它们没有自然人身份。
	//
	// ⚠️ 只能填**内部 RPC 直连**的 PSM。它取自 kitex 的 caller 字段，由框架按调用方身份填充，
	// 调用方无法在业务参数里伪造；这与 trigger_type 有本质区别 —— 后者是请求体里的普通字段，
	// 任何人都能自称 "evalx"，因此**绝不能**用 trigger_type 当授权判据。
	CallerPSMs []string `json:"caller_psms" mapstructure:"caller_psms"`
	// AllowAll 全部放行。仅用于"暂时不限制"的过渡期，不建议长期开启。
	AllowAll bool `json:"allow_all" mapstructure:"allow_all"`
}

// ConfigIDList 是人工维护配置里的 ID 列表，**数字与字符串两种写法都接受**。
//
// 为什么需要它：19 位雪花 ID（空间 ID / 用户 ID）超出 IEEE-754 float64 安全整数范围
// （2^53 ≈ 9.007e15），而配置的读写两侧对"该写哪种形态"的要求恰好相反：
//
//	写入侧：`bytedcli tcc config update` 把 JSON number 当 double 处理，
//	        7533128632407949313 会被静默截断成 7533128632407949000（实测）
//	读取侧：服务端用 encoding/json 直接解到 []int64，裸数字精确无损，
//	        但字符串形态会直接报 cannot unmarshal string into int64
//
// 也就是说：为了写得进去要用字符串，为了读得出来要用数字。任何一种单一形态都会在
// 某一侧出问题，而两种失败都**不报错给运维**（写入侧静默截断、读取侧整份配置解析失败后
// 回落到缺省值），现象都是"配了却不生效"，且截断后的 ID 与原值肉眼几乎一样，极难反推。
//
// 因此这里两种都吃：运维怎么写都对。
type ConfigIDList []int64

// UnmarshalJSON 用 strconv.ParseInt 而非 unmarshal 到 float64 再转，
// 后者会在解析阶段就丢掉精度 —— 那正是本类型要防的问题。
func (l *ConfigIDList) UnmarshalJSON(data []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	out := make(ConfigIDList, 0, len(raw))
	for _, item := range raw {
		s := strings.TrimSpace(string(item))
		// 去掉字符串形态的引号；裸数字保持原样。
		if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
			s = strings.TrimSpace(s[1 : len(s)-1])
		}
		if s == "" {
			continue
		}
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid id %q in config id list: %w", s, err)
		}
		out = append(out, id)
	}

	*l = out
	return nil
}

// MarshalJSON 统一输出字符串形态：服务若把配置回写或转发给下游，
// 字符串能避免在那一跳再被某个把 JSON number 当 double 的工具截断。
func (l ConfigIDList) MarshalJSON() ([]byte, error) {
	out := make([]string, 0, len(l))
	for _, id := range l {
		out = append(out, strconv.FormatInt(id, 10))
	}
	return json.Marshal(out)
}

// DefaultExptPriorityWhiteList 配置缺失或解析失败时的兜底：**谁都不许指定**。
//
// 取禁止而非放行：priority 是插队能力，配置中心抖动时宁可让所有人退回 default（表现为
// "我设的优先级没生效"，可见且无损），也不要因为读不到配置就让所有人都能插队
// （静默、且要等资源被抢占才发现）。
func DefaultExptPriorityWhiteList() *ExptPriorityWhiteList {
	return &ExptPriorityWhiteList{}
}

// ExptPrioritySubject 是判定的输入：一次创建实验请求里与"能否指定 priority"有关的全部身份。
//
// 收成一个结构体而不是散着传：三者都是可选的（自然人调用时 CallerPSM 为空，
// 系统调用时 UserID 可能为空），散着传容易在新增维度时漏掉调用点。
type ExptPrioritySubject struct {
	// UserEmail 已验证的用户邮箱（来自 session）。空串表示拿不到身份。
	UserEmail string
	SpaceID   int64
	CallerPSM string
}

// AllowSpecifyPriority 报告该请求是否可以指定调度优先级。
//
// nil 白名单返回 false（与本仓其它白名单一致：没配置就是没开）。
func (w *ExptPriorityWhiteList) AllowSpecifyPriority(subject ExptPrioritySubject) bool {
	if w == nil {
		return false
	}
	if w.AllowAll {
		return true
	}
	if w.matchUserEmail(subject.UserEmail) {
		return true
	}
	// spaceID=0 表示"无空间上下文"，一律不匹配 —— 否则运维在 space_ids 里误填 0
	// 就会把所有无空间上下文的请求放行。
	if subject.SpaceID != 0 && slices.Contains(w.SpaceIDs, subject.SpaceID) {
		return true
	}
	return w.matchCallerPSM(subject.CallerPSM)
}

// matchUserEmail 忽略大小写与首尾空白比对（邮箱本身大小写不敏感，且名单由人手写）。
//
// 空邮箱一律不匹配：拿不到身份时不得放行 —— 不把"读不到"当成有权限。
// 注意商业版只在 ByteTIM ticket 存在时才填 Email，所以非用户态调用（纯服务间 RPC）
// 这里必然为空，那种情况该走 CallerPSMs 维度。
func (w *ExptPriorityWhiteList) matchUserEmail(userEmail string) bool {
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
// 空 caller 一律不匹配：非 RPC 直连（HTTP 入口）时 caller 为空，那种情况该走 user/space 维度。
func (w *ExptPriorityWhiteList) matchCallerPSM(callerPSM string) bool {
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

// ExptTriggerTrustConf 决定"谁可以自称 EvalX 从而让实验进入 enforce"。
//
// 为什么需要：enforce 的第一道闸 ShouldEnforceByTrigger 只比对请求体里的 trigger_type
// 字符串，而那是调用方**自己填的普通字段** —— 任何人都能自称 "evalx"。也就是说
// "谁被中心调度纳管"实际上部分取决于调用方自报，而它本该完全由我们决定。
//
// 本表把 trigger 判据从"信自报"改成"信 RPC 框架填充的 caller"：声明 evalx 的请求
// 必须同时来自名单内的 PSM，否则该 trigger 不被采信、实验退回 legacy。
//
// ⚠️ 为什么缺省是**放行**（与 ExptPriorityWhiteList 的缺省拒绝相反）：两者失败代价不同。
// priority 配不到 → 大家退回缺省优先级，可见且无损；而 trigger 若配不到就一律拒绝，
// 会让**全部 EvalX 实验静默退回 legacy** —— 中心调度突然没有任何候选，
// 现象是"实验都在跑但一个都不受额度管控"，比"配了没生效"隐蔽得多。
// 因此取"未配置=不额外收紧"，范围仍由灰度 TCC（第二道闸）兜住，
// 等 PSM 名单在灰度环境确认无误后再打开 Enabled。
type ExptTriggerTrustConf struct {
	// Enabled 是否启用 caller 校验。false（缺省）时保持原行为：只看 trigger_type 字段。
	//
	// 用独立开关而不是"名单非空即启用"：后者会让"配置读取失败返回空名单"与
	// "运维故意留空"这两种情况行为一致，而它们的正确行为恰好相反。
	Enabled bool `json:"enabled" mapstructure:"enabled"`
	// EvalxCallerPSMs 允许声明 trigger_type=evalx 的调用方 PSM。
	EvalxCallerPSMs []string `json:"evalx_caller_psms" mapstructure:"evalx_caller_psms"`
}

// DefaultExptTriggerTrustConf 缺省：不启用 caller 校验（保持引入本闸前的行为）。
func DefaultExptTriggerTrustConf() *ExptTriggerTrustConf {
	return &ExptTriggerTrustConf{}
}

// TrustEvalxTrigger 报告该 caller 声明的 evalx trigger 是否可采信。
// 未启用时恒为 true —— 理由见类型注释里的"缺省放行"。
func (c *ExptTriggerTrustConf) TrustEvalxTrigger(callerPSM string) bool {
	if c == nil || !c.Enabled {
		return true
	}
	caller := strings.TrimSpace(callerPSM)
	if caller == "" {
		// 启用校验后拿不到 caller（非 RPC 直连）即不可采信：
		// 我们信的是框架填充的身份，没有身份就没有可信来源。
		return false
	}
	for _, psm := range c.EvalxCallerPSMs {
		if strings.EqualFold(strings.TrimSpace(psm), caller) {
			return true
		}
	}
	return false
}
