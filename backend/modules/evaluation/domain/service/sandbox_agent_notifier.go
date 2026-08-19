// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// ISandboxAgentNotifier 沙箱 agent 实验专用飞书通知。
//
// 两类卡片:
//   - 每 N 秒一张进度快照 (NotifyProgressIfDue): 由 daemon tick 反复调用, 内部用 Redis SETNX 做闸门。
//     间隔 N 从 IConfiger.GetSandboxAgentNotifyConf 读取, 支持按 space 覆盖 (兜底 1h)。
//   - 每行终态失败一张 (NotifyItemFail): CompleteItemRun fail 分支同步调用, 不限流。
//
// 两张卡都要求实验是沙箱 agent 类型, 且 NotificationConf.FeishuNotification.Enable == true。
// 非沙箱 agent / Enable=false / 接收人无法解析 时, 方法内部静默返回 nil (不阻塞主流程)。
//
// 日志前缀:
//   - 进度卡路径统一 [SandboxAgentProgress]
//   - 单行失败卡路径统一 [SandboxAgentItemFail]
//
//go:generate mockgen -destination=mocks/sandbox_agent_notifier.go -package=mocks . ISandboxAgentNotifier
type ISandboxAgentNotifier interface {
	NotifyProgressIfDue(ctx context.Context, expt *entity.Experiment) error
	NotifyItemFail(ctx context.Context, expt *entity.Experiment, itemID int64, evalErr error) error
	// ResetHourlyGate RetryAll / RetryItems 重置进度卡闸门, 让下一 tick 立刻发一张新快照。
	ResetHourlyGate(ctx context.Context, spaceID, exptID int64) error
}

// sandboxAgentItemFailErrMsgMaxLen 失败卡 err_msg 字段最大长度, 超过截断加省略号。
// 飞书卡片文本字段一般 2000 字符, 512 足够容纳一条 error 描述且留白。
const sandboxAgentItemFailErrMsgMaxLen = 512

// 卡片模板 ID 用 var 而非 const, 便于:
//   - 测试单元用例通过 test-only setter 覆写 (无需引入 build tag);
//   - 商业版部署时按 tenant/region 在启动路径注入不同 template。
//
// 默认值取自 consts, 保持与 OSS 一致 (空串 -> 静默不发)。
var (
	sandboxAgentProgressCardID = consts.SandboxAgentProgressNotifyCardID
	sandboxAgentItemFailCardID = consts.SandboxAgentItemFailNotifyCardID
)

const (
	logTagProgress = "[SandboxAgentProgress]"
	logTagItemFail = "[SandboxAgentItemFail]"
)

type sandboxAgentNotifier struct {
	notifyRPC     rpc.INotifyRPCAdapter
	userProvider  rpc.IUserProvider
	exptStatsRepo repo.IExptStatsRepo
	locker        lock.ILocker
	configer      component.IConfiger // 允许为 nil, 后续调用回落默认间隔
}

// NewSandboxAgentNotifier 构造沙箱 agent 通知器。任一依赖为 nil 时后续调用会静默 no-op。
// configer 用于读取进度卡间隔等运行期配置; nil 时全部走 entity.DefaultSandboxAgentNotifyConf。
func NewSandboxAgentNotifier(
	notifyRPC rpc.INotifyRPCAdapter,
	userProvider rpc.IUserProvider,
	exptStatsRepo repo.IExptStatsRepo,
	locker lock.ILocker,
	configer component.IConfiger,
) ISandboxAgentNotifier {
	return &sandboxAgentNotifier{
		notifyRPC:     notifyRPC,
		userProvider:  userProvider,
		exptStatsRepo: exptStatsRepo,
		locker:        locker,
		configer:      configer,
	}
}

func (s *sandboxAgentNotifier) NotifyProgressIfDue(ctx context.Context, expt *entity.Experiment) error {
	if !s.enabled(expt, logTagProgress) {
		return nil
	}
	// 卡片模板未配置时静默跳过, 避免打无效 RPC。card id 是常量, 状态不会在运行期变, 无需日志。
	if sandboxAgentProgressCardID == "" {
		return nil
	}

	// N 秒闸门: Lock SETNX + TTL=N。拿到锁 → 距上次通知≥N → 发送; 拿不到 → 静默。
	// gate 未拿到锁属于正常节流分支, daemon tick 每 10s 打一次噪声太大, 不再打日志。
	interval := s.progressNotifyInterval(ctx, expt.SpaceID)
	key := sandboxAgentProgressGateKey(expt.ID)
	locked, err := s.locker.Lock(ctx, key, interval)
	if err != nil {
		logs.CtxWarn(ctx, "%s gate lock err, expt_id=%v, interval=%v, err=%v", logTagProgress, expt.ID, interval, err)
		return nil
	}
	if !locked {
		return nil
	}

	receiveID, receiveIDType := resolveNotifyTarget(ctx, s.userProvider, expt)
	if receiveID == "" {
		logs.CtxWarn(ctx, "%s notify without target, expt_id=%v", logTagProgress, expt.ID)
		return nil
	}

	stats, err := s.exptStatsRepo.Get(ctx, expt.ID, expt.SpaceID)
	if err != nil {
		logs.CtxWarn(ctx, "%s stats get err, expt_id=%v, err=%v", logTagProgress, expt.ID, err)
		// 拿不到 stats 也发一张空快照, 便于运维知道实验还在跑; 但把 total 记 0。
	}

	param := buildSandboxAgentProgressParam(expt, stats)
	if err := s.notifyRPC.SendMessageCard(ctx, receiveID, receiveIDType, sandboxAgentProgressCardID, param); err != nil {
		logs.CtxWarn(ctx, "%s SendMessageCard err, expt_id=%v, err=%v", logTagProgress, expt.ID, err)
		return nil
	}
	logs.CtxInfo(ctx, "%s card sent, expt_id=%v, interval=%v, receive_id_type=%v, param=%v",
		logTagProgress, expt.ID, interval, receiveIDType, param)
	return nil
}

func (s *sandboxAgentNotifier) NotifyItemFail(ctx context.Context, expt *entity.Experiment, itemID int64, evalErr error) error {
	if !s.enabled(expt, logTagItemFail) {
		return nil
	}
	if sandboxAgentItemFailCardID == "" {
		return nil
	}

	receiveID, receiveIDType := resolveNotifyTarget(ctx, s.userProvider, expt)
	if receiveID == "" {
		logs.CtxWarn(ctx, "%s notify without target, expt_id=%v, item_id=%v", logTagItemFail, expt.ID, itemID)
		return nil
	}

	param := buildSandboxAgentItemFailParam(expt, itemID, evalErr)
	if err := s.notifyRPC.SendMessageCard(ctx, receiveID, receiveIDType, sandboxAgentItemFailCardID, param); err != nil {
		logs.CtxWarn(ctx, "%s SendMessageCard err, expt_id=%v, item_id=%v, err=%v", logTagItemFail, expt.ID, itemID, err)
		return nil
	}
	logs.CtxInfo(ctx, "%s card sent, expt_id=%v, item_id=%v, receive_id_type=%v", logTagItemFail, expt.ID, itemID, receiveIDType)
	return nil
}

func (s *sandboxAgentNotifier) ResetHourlyGate(ctx context.Context, spaceID, exptID int64) error {
	key := sandboxAgentProgressGateKey(exptID)
	// 用 UnlockForce 而非 Unlock: 闸门可能在不同进程/holder 上设置, holder 校验会失败。
	if _, err := s.locker.UnlockForce(ctx, key); err != nil {
		logs.CtxWarn(ctx, "%s reset gate err, expt_id=%v, err=%v", logTagProgress, exptID, err)
		return err
	}
	logs.CtxInfo(ctx, "%s gate reset, expt_id=%v", logTagProgress, exptID)
	return nil
}

// enabled 沙箱 agent + FeishuNotification.Enable 均满足才发。
// tag 用于在 skip 分支打日志时区分是哪张卡的调用方。
func (s *sandboxAgentNotifier) enabled(expt *entity.Experiment, tag string) bool {
	if s == nil || s.notifyRPC == nil {
		logs.CtxInfo(context.Background(), "%s skip: notifier or notifyRPC nil", tag)
		return false
	}
	if expt == nil {
		return false
	}
	if !isSandboxAgentExperiment(expt) {
		logs.CtxInfo(context.Background(), "%s skip: not sandbox agent experiment, expt_id=%v", tag, expt.ID)
		return false
	}
	if expt.NotificationConf == nil || expt.NotificationConf.FeishuNotification == nil {
		logs.CtxInfo(context.Background(), "%s skip: notification_conf.feishu missing, expt_id=%v", tag, expt.ID)
		return false
	}
	if !expt.NotificationConf.FeishuNotification.Enable {
		logs.CtxInfo(context.Background(), "%s skip: feishu.enable=false, expt_id=%v", tag, expt.ID)
		return false
	}
	return true
}

// progressNotifyInterval 从 configer 读进度卡间隔; configer 未注入或读失败时用默认。
func (s *sandboxAgentNotifier) progressNotifyInterval(ctx context.Context, spaceID int64) time.Duration {
	var cfg *entity.SandboxAgentNotifyConf
	if s.configer != nil {
		cfg = s.configer.GetSandboxAgentNotifyConf(ctx)
	}
	sec := cfg.GetProgressNotifyIntervalSec(spaceID) // nil-safe
	return time.Duration(sec) * time.Second
}

func sandboxAgentProgressGateKey(exptID int64) string {
	return fmt.Sprintf("sandbox_agent_progress_notify:%d", exptID)
}

func buildSandboxAgentProgressParam(expt *entity.Experiment, stats *entity.ExptStats) map[string]string {
	var success, fail, processing, pending, terminated int32
	if stats != nil {
		success = stats.SuccessItemCnt
		fail = stats.FailItemCnt
		processing = stats.ProcessingItemCnt
		pending = stats.PendingItemCnt
		terminated = stats.TerminatedItemCnt
	}
	total := success + fail + processing + pending + terminated
	return map[string]string{
		consts.SandboxAgentProgressKeyExptName:      expt.Name,
		consts.SandboxAgentProgressKeyExptID:        strconv.FormatInt(expt.ID, 10),
		consts.SandboxAgentProgressKeySpaceID:       strconv.FormatInt(expt.SpaceID, 10),
		consts.SandboxAgentProgressKeySuccessCnt:    strconv.FormatInt(int64(success), 10),
		consts.SandboxAgentProgressKeyFailCnt:       strconv.FormatInt(int64(fail), 10),
		consts.SandboxAgentProgressKeyProcessingCnt: strconv.FormatInt(int64(processing), 10),
		consts.SandboxAgentProgressKeyPendingCnt:    strconv.FormatInt(int64(pending), 10),
		consts.SandboxAgentProgressKeyTerminatedCnt: strconv.FormatInt(int64(terminated), 10),
		consts.SandboxAgentProgressKeyTotalCnt:      strconv.FormatInt(int64(total), 10),
	}
}

func buildSandboxAgentItemFailParam(expt *entity.Experiment, itemID int64, evalErr error) map[string]string {
	msg := userVisibleErrMsg(evalErr)
	if len(msg) > sandboxAgentItemFailErrMsgMaxLen {
		msg = msg[:sandboxAgentItemFailErrMsgMaxLen] + "..."
	}
	return map[string]string{
		consts.SandboxAgentItemFailKeyExptName: expt.Name,
		consts.SandboxAgentItemFailKeyExptID:   strconv.FormatInt(expt.ID, 10),
		consts.SandboxAgentItemFailKeyItemID:   strconv.FormatInt(itemID, 10),
		consts.SandboxAgentItemFailKeySpaceID:  strconv.FormatInt(expt.SpaceID, 10),
		consts.SandboxAgentItemFailKeyErrMsg:   msg,
	}
}
