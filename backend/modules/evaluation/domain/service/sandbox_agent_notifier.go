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
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// ISandboxAgentNotifier 沙箱 agent 实验专用飞书通知。
//
// 两类卡片:
//   - 每 1h 一张进度快照 (NotifyProgressIfDue): 由 daemon tick 反复调用, 内部用 Redis SETNX 做闸门。
//   - 每行终态失败一张 (NotifyItemFail): CompleteItemRun fail 分支同步调用, 不限流。
//
// 两张卡都要求实验是沙箱 agent 类型, 且 NotificationConf.FeishuNotification.Enable == true。
// 非沙箱 agent / Enable=false / 接收人无法解析 时, 方法内部静默返回 nil (不阻塞主流程)。
type ISandboxAgentNotifier interface {
	NotifyProgressIfDue(ctx context.Context, expt *entity.Experiment) error
	NotifyItemFail(ctx context.Context, expt *entity.Experiment, itemID int64, evalErr error) error
	// ResetHourlyGate RetryAll / RetryItems 重置 1h 闸门, 让下一 tick 立刻发一张新快照。
	ResetHourlyGate(ctx context.Context, spaceID, exptID int64) error
}

// sandboxAgentProgressGateTTL 每 1h 进度卡的闸门 TTL。同 key 内不再重复发。
const sandboxAgentProgressGateTTL = time.Hour

// sandboxAgentItemFailErrMsgMaxLen 失败卡 err_msg 字段最大长度, 超过截断加省略号。
// 飞书卡片文本字段一般 2000 字符, 512 足够容纳一条 error 描述且留白。
const sandboxAgentItemFailErrMsgMaxLen = 512

type sandboxAgentNotifier struct {
	notifyRPC     rpc.INotifyRPCAdapter
	userProvider  rpc.IUserProvider
	exptStatsRepo repo.IExptStatsRepo
	locker        lock.ILocker
}

// NewSandboxAgentNotifier 构造沙箱 agent 通知器。任一依赖为 nil 时后续调用会静默 no-op。
func NewSandboxAgentNotifier(
	notifyRPC rpc.INotifyRPCAdapter,
	userProvider rpc.IUserProvider,
	exptStatsRepo repo.IExptStatsRepo,
	locker lock.ILocker,
) ISandboxAgentNotifier {
	return &sandboxAgentNotifier{
		notifyRPC:     notifyRPC,
		userProvider:  userProvider,
		exptStatsRepo: exptStatsRepo,
		locker:        locker,
	}
}

func (s *sandboxAgentNotifier) NotifyProgressIfDue(ctx context.Context, expt *entity.Experiment) error {
	if !s.enabled(expt) {
		return nil
	}
	// 卡片模板未配置时静默跳过, 避免打无效 RPC。
	if consts.SandboxAgentProgressNotifyCardID == "" {
		return nil
	}

	// 1h 闸门: Lock SETNX + TTL=1h。拿到锁 → 距上次通知≥1h → 发送; 拿不到 → 静默。
	key := sandboxAgentProgressGateKey(expt.ID)
	locked, err := s.locker.Lock(ctx, key, sandboxAgentProgressGateTTL)
	if err != nil {
		logs.CtxWarn(ctx, "[SandboxAgentNotify] progress gate lock err, expt_id=%v, err=%v", expt.ID, err)
		return nil
	}
	if !locked {
		return nil
	}

	receiveID, receiveIDType := resolveNotifyTarget(ctx, s.userProvider, expt)
	if receiveID == "" {
		logs.CtxWarn(ctx, "[SandboxAgentNotify] progress notify without target, expt_id=%v", expt.ID)
		return nil
	}

	stats, err := s.exptStatsRepo.Get(ctx, expt.ID, expt.SpaceID)
	if err != nil {
		logs.CtxWarn(ctx, "[SandboxAgentNotify] progress stats get err, expt_id=%v, err=%v", expt.ID, err)
		// 拿不到 stats 也发一张空快照, 便于运维知道实验还在跑; 但把 total 记 0。
	}

	param := buildSandboxAgentProgressParam(expt, stats)
	if err := s.notifyRPC.SendMessageCard(ctx, receiveID, receiveIDType, consts.SandboxAgentProgressNotifyCardID, param); err != nil {
		logs.CtxWarn(ctx, "[SandboxAgentNotify] progress SendMessageCard err, expt_id=%v, err=%v", expt.ID, err)
		return nil
	}
	logs.CtxInfo(ctx, "[SandboxAgentNotify] progress card sent, expt_id=%v, param=%v", expt.ID, param)
	return nil
}

func (s *sandboxAgentNotifier) NotifyItemFail(ctx context.Context, expt *entity.Experiment, itemID int64, evalErr error) error {
	if !s.enabled(expt) {
		return nil
	}
	if consts.SandboxAgentItemFailNotifyCardID == "" {
		return nil
	}

	receiveID, receiveIDType := resolveNotifyTarget(ctx, s.userProvider, expt)
	if receiveID == "" {
		logs.CtxWarn(ctx, "[SandboxAgentNotify] item fail notify without target, expt_id=%v, item_id=%v", expt.ID, itemID)
		return nil
	}

	param := buildSandboxAgentItemFailParam(expt, itemID, evalErr)
	if err := s.notifyRPC.SendMessageCard(ctx, receiveID, receiveIDType, consts.SandboxAgentItemFailNotifyCardID, param); err != nil {
		logs.CtxWarn(ctx, "[SandboxAgentNotify] item fail SendMessageCard err, expt_id=%v, item_id=%v, err=%v", expt.ID, itemID, err)
		return nil
	}
	logs.CtxInfo(ctx, "[SandboxAgentNotify] item fail card sent, expt_id=%v, item_id=%v", expt.ID, itemID)
	return nil
}

func (s *sandboxAgentNotifier) ResetHourlyGate(ctx context.Context, spaceID, exptID int64) error {
	key := sandboxAgentProgressGateKey(exptID)
	// 用 UnlockForce 而非 Unlock: 闸门可能在不同进程/holder 上设置, holder 校验会失败。
	if _, err := s.locker.UnlockForce(ctx, key); err != nil {
		logs.CtxWarn(ctx, "[SandboxAgentNotify] reset hourly gate err, expt_id=%v, err=%v", exptID, err)
		return err
	}
	logs.CtxInfo(ctx, "[SandboxAgentNotify] hourly gate reset, expt_id=%v", exptID)
	return nil
}

// enabled 沙箱 agent + FeishuNotification.Enable 均满足才发。
func (s *sandboxAgentNotifier) enabled(expt *entity.Experiment) bool {
	if s == nil || s.notifyRPC == nil {
		return false
	}
	if !isSandboxAgentExperiment(expt) {
		return false
	}
	if expt.NotificationConf == nil || expt.NotificationConf.FeishuNotification == nil {
		return false
	}
	return expt.NotificationConf.FeishuNotification.Enable
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
