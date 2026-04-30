// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// scheduleCallbackMethod ByteScheduler 触发时回调的 RPC 方法名（OpenAPI 服务对外的根据模板提交实验接口）
const scheduleCallbackMethod = "SubmitExptFromTemplateOApi"

// scheduleBizKeyPrefix BizKey 前缀，便于在调度平台侧按业务区分定时任务来源
const scheduleBizKeyPrefix = "expt_template_schedule"

// schedulerCallbackPayload 调用 SubmitExptFromTemplateOApi 的 JSON Body
//
// 与 SubmitExptFromTemplateOApiRequest 中 api.body+api.js_conv 保持一致：
// i64 字段以字符串形式序列化，避免 JS 端精度丢失。
type schedulerCallbackPayload struct {
	WorkspaceID string `json:"workspace_id"`
	TemplateID  string `json:"template_id"`
}

// syncSchedulerForTemplate 根据实验模板的 ExptSource.Scheduler 同步底层调度任务。
//
// 仅当 SourceType=Evaluation 时启用周期调度；其它来源（AutoTask/Workflow/IntelligentGen）
// 由各自上游驱动，不在此处管理。
//
// 行为约定：
//   - Scheduler 启用且配置完整 → CreatePeriodicJob（同 BizKey 重复调用为 upsert）
//   - Scheduler 未启用、配置不完整或 ExptSource 为空 → CloseJob（已关闭/不存在则幂等）
//   - 任何错误仅记录日志，不阻断模板创建/更新主流程
func (e *ExptTemplateManagerImpl) syncSchedulerForTemplate(ctx context.Context, template *entity.ExptTemplate) {
	if e == nil || e.scheduleAdapter == nil || template == nil {
		logs.CtxWarn(ctx, "[expt_template_sched] skip sync: nil receiver/adapter/template (e_nil=%v, adapter_nil=%v, template_nil=%v)",
			e == nil, e != nil && e.scheduleAdapter == nil, template == nil)
		return
	}
	templateID := template.GetID()
	spaceID := template.GetSpaceID()
	if templateID <= 0 || spaceID <= 0 {
		logs.CtxWarn(ctx, "[expt_template_sched] skip sync: invalid id, space_id=%d, template_id=%d", spaceID, templateID)
		return
	}

	bizKey := buildScheduleBizKey(spaceID, templateID)
	source := template.ExptSource
	cronActivate := template.ExptInfo != nil && template.ExptInfo.CronActivate

	// 入口日志：先把所有判定输入打齐，便于一行定位"为什么没下发"
	logs.CtxInfo(ctx, "[expt_template_sched] start sync, biz_key=%s, source_type=%v, cron_activate=%v, has_scheduler=%v, scheduler=%s",
		bizKey,
		schedSourceTypeName(source),
		cronActivate,
		source != nil && source.Scheduler != nil,
		schedDescribeScheduler(source))

	// 非 Evaluation 来源不接管定时调度；同时清理可能遗留的任务以避免误触发
	if source == nil || source.SourceType != entity.SourceType_Evaluation {
		logs.CtxInfo(ctx, "[expt_template_sched] non-evaluation source, will close any existing job, biz_key=%s, source_type=%v",
			bizKey, schedSourceTypeName(source))
		if err := e.scheduleAdapter.CloseJob(ctx, bizKey); err != nil {
			logs.CtxWarn(ctx, "[expt_template_sched] close schedule job failed (non-evaluation source), biz_key=%s, err=%v", bizKey, err)
		}
		return
	}

	// 模板未启用 cron 或 Scheduler 配置缺失/未启用 → 关闭已存在任务
	if !cronActivate || source.Scheduler == nil || !isSchedulerEnabled(source.Scheduler) {
		reason := schedDisabledReason(cronActivate, source.Scheduler)
		logs.CtxInfo(ctx, "[expt_template_sched] schedule disabled, will close any existing job, biz_key=%s, reason=%s",
			bizKey, reason)
		if err := e.scheduleAdapter.CloseJob(ctx, bizKey); err != nil {
			logs.CtxWarn(ctx, "[expt_template_sched] close schedule job failed, biz_key=%s, reason=%s, err=%v", bizKey, reason, err)
		}
		return
	}

	param, err := buildCreatePeriodicJobParam(bizKey, spaceID, templateID, source.Scheduler)
	if err != nil {
		logs.CtxError(ctx, "[expt_template_sched] build create periodic job param failed, biz_key=%s, err=%v", bizKey, err)
		return
	}
	logs.CtxInfo(ctx, "[expt_template_sched] dispatch CreatePeriodicJob, biz_key=%s, crontab=%s, started_at=%v, ended_at=%v, callback=%s, payload=%s",
		param.BizKey, param.Crontab, param.StartedAt, param.EndedAt, param.CallbackMethod, param.CallbackPayload)
	if err := e.scheduleAdapter.CreatePeriodicJob(ctx, param); err != nil {
		logs.CtxError(ctx, "[expt_template_sched] create periodic schedule job failed, biz_key=%s, crontab=%s, err=%v", bizKey, param.Crontab, err)
		return
	}
	logs.CtxInfo(ctx, "[expt_template_sched] schedule job synced, biz_key=%s, frequency=%s, crontab=%s",
		bizKey, *source.Scheduler.Frequency, param.Crontab)
}

// schedSourceTypeName 把 SourceType 打成可读字符串，便于日志快速定位
func schedSourceTypeName(src *entity.ExptSource) string {
	if src == nil {
		return "<nil_source>"
	}
	return fmt.Sprintf("%d", src.SourceType)
}

// schedDescribeScheduler 把 Scheduler 字段一行展开，所有可空字段都安全打印
func schedDescribeScheduler(src *entity.ExptSource) string {
	if src == nil || src.Scheduler == nil {
		return "<nil_scheduler>"
	}
	s := src.Scheduler
	enabled := false
	if s.Enabled != nil {
		enabled = *s.Enabled
	}
	freq := ""
	if s.Frequency != nil {
		freq = *s.Frequency
	}
	var trigger, start, end int64
	if s.TriggerAt != nil {
		trigger = *s.TriggerAt
	}
	if s.StartTime != nil {
		start = *s.StartTime
	}
	if s.EndTime != nil {
		end = *s.EndTime
	}
	return fmt.Sprintf("{enabled=%v,frequency=%q,trigger_at=%d,start_time=%d,end_time=%d}",
		enabled, freq, trigger, start, end)
}

// schedDisabledReason 推导本次"未下发周期任务"的具体原因，便于排查
func schedDisabledReason(cronActivate bool, s *entity.ExptSchedulerDO) string {
	if !cronActivate {
		return "expt_info.cron_activate=false"
	}
	if s == nil {
		return "scheduler is nil (DTO 转换/落库链路没有保留 Scheduler？)"
	}
	if s.Enabled == nil || !*s.Enabled {
		return "scheduler.enabled=false"
	}
	if s.Frequency == nil || *s.Frequency == "" {
		return "scheduler.frequency is empty"
	}
	if s.TriggerAt == nil || *s.TriggerAt <= 0 {
		return "scheduler.trigger_at is empty"
	}
	return "unknown"
}

// isSchedulerEnabled 判断 ExptSchedulerDO 是否启用且配置完整
func isSchedulerEnabled(s *entity.ExptSchedulerDO) bool {
	if s == nil {
		return false
	}
	if s.Enabled == nil || !*s.Enabled {
		return false
	}
	if s.Frequency == nil || *s.Frequency == "" {
		return false
	}
	if s.TriggerAt == nil || *s.TriggerAt <= 0 {
		return false
	}
	return true
}

// buildScheduleBizKey 构造 BizKey：expt_template_schedule:{spaceID}:{templateID}
func buildScheduleBizKey(spaceID, templateID int64) string {
	return fmt.Sprintf("%s:%d:%d", scheduleBizKeyPrefix, spaceID, templateID)
}

// buildCreatePeriodicJobParam 由 ExptSchedulerDO 推导出 CreatePeriodicJobParam
func buildCreatePeriodicJobParam(bizKey string, spaceID, templateID int64, s *entity.ExptSchedulerDO) (*rpc.CreatePeriodicJobParam, error) {
	crontab, err := schedulerToCrontab(s)
	if err != nil {
		return nil, err
	}
	payload, err := buildSchedulerCallbackPayload(spaceID, templateID)
	if err != nil {
		return nil, err
	}
	param := &rpc.CreatePeriodicJobParam{
		BizKey:          bizKey,
		Crontab:         crontab,
		CallbackMethod:  scheduleCallbackMethod,
		CallbackPayload: payload,
	}
	if s.StartTime != nil && *s.StartTime > 0 {
		t := time.Unix(*s.StartTime, 0)
		param.StartedAt = &t
	}
	if s.EndTime != nil && *s.EndTime > 0 {
		t := time.Unix(*s.EndTime, 0)
		param.EndedAt = &t
	}
	return param, nil
}

// schedulerToCrontab 将 ExptSchedulerDO 推导出标准 5 段 crontab（minute hour day-of-month month day-of-week）
//
// TriggerAt 为时间戳但仅取时分；按服务器本地时区解析。
func schedulerToCrontab(s *entity.ExptSchedulerDO) (string, error) {
	if s == nil || s.TriggerAt == nil || s.Frequency == nil {
		return "", errorx.New("scheduler trigger_at/frequency is required")
	}
	trigger := time.Unix(*s.TriggerAt, 0).Local()
	dow, err := frequencyToDayOfWeek(*s.Frequency)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d %d * * %s", trigger.Minute(), trigger.Hour(), dow), nil
}

// frequencyToDayOfWeek 将 Frequency 取值映射到 crontab 的 day-of-week 字段
//
// crontab 周字段约定：0/7=Sunday, 1=Monday, ..., 6=Saturday
func frequencyToDayOfWeek(frequency string) (string, error) {
	switch frequency {
	case entity.FrequencyEveryDay:
		return "*", nil
	case entity.FrequencyMonday:
		return "1", nil
	case entity.FrequencyTuesday:
		return "2", nil
	case entity.FrequencyWednesday:
		return "3", nil
	case entity.FrequencyThursday:
		return "4", nil
	case entity.FrequencyFriday:
		return "5", nil
	case entity.FrequencySaturday:
		return "6", nil
	case entity.FrequencySunday:
		return "0", nil
	default:
		return "", errorx.New("unsupported scheduler frequency: %s", frequency)
	}
}

// buildSchedulerCallbackPayload 序列化 ByteScheduler 触发时的 RPC Body
func buildSchedulerCallbackPayload(spaceID, templateID int64) (string, error) {
	body := schedulerCallbackPayload{
		WorkspaceID: fmt.Sprintf("%d", spaceID),
		TemplateID:  fmt.Sprintf("%d", templateID),
	}
	bs, err := json.Marshal(body)
	if err != nil {
		return "", errorx.Wrapf(err, "marshal scheduler callback payload fail")
	}
	return string(bs), nil
}
