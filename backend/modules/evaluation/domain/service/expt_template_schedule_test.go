// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

// ---------- 纯函数 ----------

func TestFrequencyToDayOfWeek(t *testing.T) {
	cases := []struct {
		in        string
		want      string
		expectErr bool
	}{
		{entity.FrequencyEveryDay, "*", false},
		{entity.FrequencyMonday, "1", false},
		{entity.FrequencyTuesday, "2", false},
		{entity.FrequencyWednesday, "3", false},
		{entity.FrequencyThursday, "4", false},
		{entity.FrequencyFriday, "5", false},
		{entity.FrequencySaturday, "6", false},
		{entity.FrequencySunday, "0", false},
		{"", "", true},
		{"unknown", "", true},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, err := frequencyToDayOfWeek(c.in)
			if c.expectErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, c.want, got)
		})
	}
}

func TestSchedulerToCrontab(t *testing.T) {
	t.Run("nil scheduler", func(t *testing.T) {
		_, err := schedulerToCrontab(nil)
		assert.Error(t, err)
	})

	t.Run("missing trigger_at", func(t *testing.T) {
		_, err := schedulerToCrontab(&entity.ExptSchedulerDO{
			Frequency: gptr.Of(entity.FrequencyEveryDay),
		})
		assert.Error(t, err)
	})

	t.Run("missing frequency", func(t *testing.T) {
		_, err := schedulerToCrontab(&entity.ExptSchedulerDO{
			TriggerAt: gptr.Of(int64(0)),
		})
		// TriggerAt=0 也是非法（trigger_at <=0），由 isSchedulerEnabled 拦截；这里仅测 nil frequency
		assert.Error(t, err)
	})

	t.Run("every_day uses *", func(t *testing.T) {
		// 选一个本地时区下时分稳定的时间戳：基于"今天 09:30"构造
		now := time.Now().Local()
		trigger := time.Date(now.Year(), now.Month(), now.Day(), 9, 30, 0, 0, time.Local)
		s := &entity.ExptSchedulerDO{
			TriggerAt: gptr.Of(trigger.Unix()),
			Frequency: gptr.Of(entity.FrequencyEveryDay),
		}
		cron, err := schedulerToCrontab(s)
		assert.NoError(t, err)
		assert.Equal(t, "30 9 * * *", cron)
	})

	t.Run("monday weekly", func(t *testing.T) {
		now := time.Now().Local()
		trigger := time.Date(now.Year(), now.Month(), now.Day(), 0, 5, 0, 0, time.Local)
		s := &entity.ExptSchedulerDO{
			TriggerAt: gptr.Of(trigger.Unix()),
			Frequency: gptr.Of(entity.FrequencyMonday),
		}
		cron, err := schedulerToCrontab(s)
		assert.NoError(t, err)
		assert.Equal(t, "5 0 * * 1", cron)
	})

	t.Run("invalid frequency", func(t *testing.T) {
		s := &entity.ExptSchedulerDO{
			TriggerAt: gptr.Of(int64(1)),
			Frequency: gptr.Of("not_a_freq"),
		}
		_, err := schedulerToCrontab(s)
		assert.Error(t, err)
	})
}

func TestBuildScheduleBizKey(t *testing.T) {
	got := buildScheduleBizKey(123, 456)
	assert.Equal(t, "expt_template_schedule:123:456", got)
	assert.True(t, strings.HasPrefix(got, scheduleBizKeyPrefix+":"))
}

func TestBuildSchedulerCallbackPayload(t *testing.T) {
	payload, err := buildSchedulerCallbackPayload(100, 200)
	assert.NoError(t, err)

	// 反序列化校验，避免依赖序列化字段顺序
	var got schedulerCallbackPayload
	assert.NoError(t, json.Unmarshal([]byte(payload), &got))
	assert.Equal(t, "100", got.WorkspaceID)
	assert.Equal(t, "200", got.TemplateID)

	// i64 字段以字符串形式序列化（兼容前端 BigInt 精度）
	assert.True(t, strings.Contains(payload, `"workspace_id":"100"`))
	assert.True(t, strings.Contains(payload, `"template_id":"200"`))
}

func TestBuildCreatePeriodicJobParam(t *testing.T) {
	t.Run("full case", func(t *testing.T) {
		now := time.Now().Local()
		trigger := time.Date(now.Year(), now.Month(), now.Day(), 8, 0, 0, 0, time.Local)
		start := trigger.Add(time.Hour)
		end := trigger.Add(24 * time.Hour)

		s := &entity.ExptSchedulerDO{
			Enabled:   gptr.Of(true),
			Frequency: gptr.Of(entity.FrequencyEveryDay),
			TriggerAt: gptr.Of(trigger.Unix()),
			StartTime: gptr.Of(start.Unix()),
			EndTime:   gptr.Of(end.Unix()),
		}

		bizKey := buildScheduleBizKey(11, 22)
		param, err := buildCreatePeriodicJobParam(bizKey, 11, 22, s)
		assert.NoError(t, err)
		assert.Equal(t, bizKey, param.BizKey)
		assert.Equal(t, "0 8 * * *", param.Crontab)
		assert.Equal(t, scheduleCallbackMethod, param.CallbackMethod)
		assert.NotEmpty(t, param.CallbackPayload)
		assert.NotNil(t, param.StartedAt)
		assert.Equal(t, start.Unix(), param.StartedAt.Unix())
		assert.NotNil(t, param.EndedAt)
		assert.Equal(t, end.Unix(), param.EndedAt.Unix())
	})

	t.Run("missing start/end => nil", func(t *testing.T) {
		now := time.Now().Local()
		trigger := time.Date(now.Year(), now.Month(), now.Day(), 1, 0, 0, 0, time.Local)
		s := &entity.ExptSchedulerDO{
			Enabled:   gptr.Of(true),
			Frequency: gptr.Of(entity.FrequencyEveryDay),
			TriggerAt: gptr.Of(trigger.Unix()),
		}
		param, err := buildCreatePeriodicJobParam("k", 1, 1, s)
		assert.NoError(t, err)
		assert.Nil(t, param.StartedAt)
		assert.Nil(t, param.EndedAt)
	})

	t.Run("start/end 为 0 视为未配置", func(t *testing.T) {
		now := time.Now().Local()
		trigger := time.Date(now.Year(), now.Month(), now.Day(), 1, 0, 0, 0, time.Local)
		s := &entity.ExptSchedulerDO{
			Enabled:   gptr.Of(true),
			Frequency: gptr.Of(entity.FrequencyEveryDay),
			TriggerAt: gptr.Of(trigger.Unix()),
			StartTime: gptr.Of(int64(0)),
			EndTime:   gptr.Of(int64(0)),
		}
		param, err := buildCreatePeriodicJobParam("k", 1, 1, s)
		assert.NoError(t, err)
		assert.Nil(t, param.StartedAt)
		assert.Nil(t, param.EndedAt)
	})

	t.Run("invalid frequency 报错", func(t *testing.T) {
		s := &entity.ExptSchedulerDO{
			Enabled:   gptr.Of(true),
			Frequency: gptr.Of("foo"),
			TriggerAt: gptr.Of(int64(1)),
		}
		_, err := buildCreatePeriodicJobParam("k", 1, 1, s)
		assert.Error(t, err)
	})
}

func TestIsSchedulerEnabled(t *testing.T) {
	cases := []struct {
		name string
		in   *entity.ExptSchedulerDO
		want bool
	}{
		{"nil", nil, false},
		{"enabled nil", &entity.ExptSchedulerDO{Frequency: gptr.Of("every_day"), TriggerAt: gptr.Of(int64(1))}, false},
		{"enabled false", &entity.ExptSchedulerDO{Enabled: gptr.Of(false), Frequency: gptr.Of("every_day"), TriggerAt: gptr.Of(int64(1))}, false},
		{"frequency empty", &entity.ExptSchedulerDO{Enabled: gptr.Of(true), Frequency: gptr.Of(""), TriggerAt: gptr.Of(int64(1))}, false},
		{"frequency nil", &entity.ExptSchedulerDO{Enabled: gptr.Of(true), TriggerAt: gptr.Of(int64(1))}, false},
		{"trigger_at zero", &entity.ExptSchedulerDO{Enabled: gptr.Of(true), Frequency: gptr.Of("every_day"), TriggerAt: gptr.Of(int64(0))}, false},
		{"trigger_at nil", &entity.ExptSchedulerDO{Enabled: gptr.Of(true), Frequency: gptr.Of("every_day")}, false},
		{"valid", &entity.ExptSchedulerDO{Enabled: gptr.Of(true), Frequency: gptr.Of("every_day"), TriggerAt: gptr.Of(int64(1))}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, isSchedulerEnabled(c.in))
		})
	}
}

// ---------- syncSchedulerForTemplate 集成 ----------

func newTemplateWithSchedule(spaceID, tplID int64, source *entity.ExptSource, cronActivate bool) *entity.ExptTemplate {
	return &entity.ExptTemplate{
		Meta: &entity.ExptTemplateMeta{
			ID:          tplID,
			WorkspaceID: spaceID,
		},
		ExptSource: source,
		ExptInfo: &entity.ExptInfo{
			CronActivate: cronActivate,
		},
	}
}

func newEnabledScheduler() *entity.ExptSchedulerDO {
	now := time.Now().Local()
	trigger := time.Date(now.Year(), now.Month(), now.Day(), 10, 30, 0, 0, time.Local)
	return &entity.ExptSchedulerDO{
		Enabled:   gptr.Of(true),
		Frequency: gptr.Of(entity.FrequencyEveryDay),
		TriggerAt: gptr.Of(trigger.Unix()),
	}
}

func TestSyncSchedulerForTemplate(t *testing.T) {
	ctx := context.Background()
	spaceID := int64(1001)
	tplID := int64(2002)
	bizKey := buildScheduleBizKey(spaceID, tplID)

	t.Run("nil receiver / adapter / template => no-op", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		// 允许 0 次调用就行；显式地构造一个 adapter，但 template=nil 时方法内部直接 return
		mockAdapter := mocks.NewMockIExptScheduleAdapter(ctrl)

		var nilMgr *ExptTemplateManagerImpl
		assert.NotPanics(t, func() {
			nilMgr.syncSchedulerForTemplate(ctx, &entity.ExptTemplate{})
		})

		mgrNoAdapter := &ExptTemplateManagerImpl{}
		mgrNoAdapter.syncSchedulerForTemplate(ctx, &entity.ExptTemplate{})

		mgr := &ExptTemplateManagerImpl{scheduleAdapter: mockAdapter}
		mgr.syncSchedulerForTemplate(ctx, nil)
	})

	t.Run("空 templateID/spaceID => no-op", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockAdapter := mocks.NewMockIExptScheduleAdapter(ctrl)
		mgr := &ExptTemplateManagerImpl{scheduleAdapter: mockAdapter}
		// Meta.ID = 0 -> GetID() 返回 0，不应触达 adapter
		mgr.syncSchedulerForTemplate(ctx, &entity.ExptTemplate{Meta: &entity.ExptTemplateMeta{WorkspaceID: 1}})
		mgr.syncSchedulerForTemplate(ctx, &entity.ExptTemplate{Meta: &entity.ExptTemplateMeta{ID: 1}})
	})

	t.Run("非 Evaluation 来源 => CloseJob", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockAdapter := mocks.NewMockIExptScheduleAdapter(ctrl)
		mockAdapter.EXPECT().CloseJob(ctx, bizKey).Return(nil)
		mgr := &ExptTemplateManagerImpl{scheduleAdapter: mockAdapter}

		tpl := newTemplateWithSchedule(spaceID, tplID, &entity.ExptSource{
			SourceType: entity.SourceType_Trace,
			Scheduler:  newEnabledScheduler(),
		}, true)
		mgr.syncSchedulerForTemplate(ctx, tpl)
	})

	t.Run("ExptSource 为 nil => CloseJob", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockAdapter := mocks.NewMockIExptScheduleAdapter(ctrl)
		mockAdapter.EXPECT().CloseJob(ctx, bizKey).Return(nil)
		mgr := &ExptTemplateManagerImpl{scheduleAdapter: mockAdapter}

		tpl := newTemplateWithSchedule(spaceID, tplID, nil, true)
		mgr.syncSchedulerForTemplate(ctx, tpl)
	})

	t.Run("CronActivate=false => CloseJob", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockAdapter := mocks.NewMockIExptScheduleAdapter(ctrl)
		mockAdapter.EXPECT().CloseJob(ctx, bizKey).Return(nil)
		mgr := &ExptTemplateManagerImpl{scheduleAdapter: mockAdapter}

		tpl := newTemplateWithSchedule(spaceID, tplID, &entity.ExptSource{
			SourceType: entity.SourceType_Evaluation,
			Scheduler:  newEnabledScheduler(),
		}, false)
		mgr.syncSchedulerForTemplate(ctx, tpl)
	})

	t.Run("Scheduler.Enabled=false => CloseJob", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockAdapter := mocks.NewMockIExptScheduleAdapter(ctrl)
		mockAdapter.EXPECT().CloseJob(ctx, bizKey).Return(nil)
		mgr := &ExptTemplateManagerImpl{scheduleAdapter: mockAdapter}

		s := newEnabledScheduler()
		s.Enabled = gptr.Of(false)
		tpl := newTemplateWithSchedule(spaceID, tplID, &entity.ExptSource{
			SourceType: entity.SourceType_Evaluation,
			Scheduler:  s,
		}, true)
		mgr.syncSchedulerForTemplate(ctx, tpl)
	})

	t.Run("CloseJob 失败仅记日志，不 panic", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockAdapter := mocks.NewMockIExptScheduleAdapter(ctrl)
		mockAdapter.EXPECT().CloseJob(ctx, bizKey).Return(errors.New("close fail"))
		mgr := &ExptTemplateManagerImpl{scheduleAdapter: mockAdapter}

		tpl := newTemplateWithSchedule(spaceID, tplID, nil, false)
		assert.NotPanics(t, func() {
			mgr.syncSchedulerForTemplate(ctx, tpl)
		})
	})

	t.Run("有效配置 => CreatePeriodicJob", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockAdapter := mocks.NewMockIExptScheduleAdapter(ctrl)

		var captured *rpc.CreatePeriodicJobParam
		mockAdapter.EXPECT().
			CreatePeriodicJob(ctx, gomock.Any()).
			DoAndReturn(func(_ context.Context, p *rpc.CreatePeriodicJobParam) error {
				captured = p
				return nil
			})

		mgr := &ExptTemplateManagerImpl{scheduleAdapter: mockAdapter}
		s := newEnabledScheduler()
		tpl := newTemplateWithSchedule(spaceID, tplID, &entity.ExptSource{
			SourceType: entity.SourceType_Evaluation,
			Scheduler:  s,
		}, true)

		mgr.syncSchedulerForTemplate(ctx, tpl)

		assert.NotNil(t, captured)
		assert.Equal(t, bizKey, captured.BizKey)
		assert.Equal(t, scheduleCallbackMethod, captured.CallbackMethod)
		assert.True(t, strings.Contains(captured.CallbackPayload, fmt.Sprintf(`"template_id":"%d"`, tplID)))
		// crontab 至少包含 5 段
		assert.Equal(t, 5, len(strings.Fields(captured.Crontab)))
	})

	t.Run("CreatePeriodicJob 失败仅记日志", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockAdapter := mocks.NewMockIExptScheduleAdapter(ctrl)
		mockAdapter.EXPECT().
			CreatePeriodicJob(ctx, gomock.Any()).
			Return(errors.New("create fail"))

		mgr := &ExptTemplateManagerImpl{scheduleAdapter: mockAdapter}
		tpl := newTemplateWithSchedule(spaceID, tplID, &entity.ExptSource{
			SourceType: entity.SourceType_Evaluation,
			Scheduler:  newEnabledScheduler(),
		}, true)
		assert.NotPanics(t, func() {
			mgr.syncSchedulerForTemplate(ctx, tpl)
		})
	})
}
