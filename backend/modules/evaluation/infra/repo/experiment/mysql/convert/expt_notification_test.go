// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

func sampleNotificationConf() *entity.NotificationConf {
	return &entity.NotificationConf{
		Filter: &entity.NotificationFilterCondition{
			FieldType: entity.FieldType_ExptStatus,
			Operator:  entity.NotificationFilterOperatorType_In,
			Values: []entity.NotificationStatusValue{
				entity.NotificationStatusValue_Started,
				entity.NotificationStatusValue_Succeeded,
				entity.NotificationStatusValue_Terminated,
			},
		},
		Webhook: &entity.WebhookNotificationConf{Enable: true, URLs: []string{"https://hook1", "https://hook2"}},
		Feishu:  &entity.FeishuNotificationConf{Enable: true},
	}
}

// 实验 notification_conf 往返：DO -> PO -> DO 应无损。
func TestExptConverter_NotificationConf_Roundtrip(t *testing.T) {
	t.Parallel()

	conv := NewExptConverter()
	do := &entity.Experiment{
		ID:               101,
		SpaceID:          7,
		Name:             "exp",
		NotificationConf: sampleNotificationConf(),
	}

	po, err := conv.DO2PO(do)
	assert.NoError(t, err)
	assert.NotNil(t, po.NotificationConf)

	got, err := conv.PO2DO(po, nil)
	assert.NoError(t, err)
	assert.Equal(t, do.NotificationConf, got.NotificationConf)
}

// 历史实验（NULL notification_conf）：PO -> DO 应得 nil，由上层 DefaultNotificationConf 兜底（向后兼容、零迁移）。
func TestExptConverter_NotificationConf_NilBackwardCompatible(t *testing.T) {
	t.Parallel()

	conv := NewExptConverter()

	// DO 侧 nil -> PO 不写列。
	po, err := conv.DO2PO(&entity.Experiment{ID: 1, Name: "x"})
	assert.NoError(t, err)
	assert.Nil(t, po.NotificationConf)

	// PO 侧 NULL（nil 指针）-> DO nil。
	got, err := conv.PO2DO(&model.Experiment{ID: 1, Name: "x"}, nil)
	assert.NoError(t, err)
	assert.Nil(t, got.NotificationConf)
	// nil 配置的运行期行为：默认飞书开启、webhook 关闭。
	assert.True(t, got.NotificationConf.GetNotificationConfOrDefault().Feishu.Enable)
	assert.False(t, got.NotificationConf.GetNotificationConfOrDefault().Webhook.Enable)
}

// PO 侧空 BLOB（长度 0）也按 NULL 处理，不报错。
func TestExptConverter_NotificationConf_EmptyBytesTreatedAsNull(t *testing.T) {
	t.Parallel()

	conv := NewExptConverter()
	empty := []byte{}
	got, err := conv.PO2DO(&model.Experiment{ID: 1, Name: "x", NotificationConf: &empty}, nil)
	assert.NoError(t, err)
	assert.Nil(t, got.NotificationConf)
}

// 脏数据/旧格式 notification_conf（webhook.urls 存成 string 而非 []string）：
// 单条 unmarshal 失败应降级为 nil 且不报错，由上层 DefaultNotificationConf 兜底，
// 不阻断该实验返回，更不让整个 list 查询失败。
func TestExptConverter_NotificationConf_DirtyDataDegradeToNil(t *testing.T) {
	t.Parallel()

	conv := NewExptConverter()
	// 复现 expt_id 7590101991789854978 的脏数据：webhook.urls 是字符串而非数组。
	dirty := []byte(`{"Webhook":{"Enable":true,"URLs":"efwefewfewf"}}`)
	got, err := conv.PO2DO(&model.Experiment{ID: 7590101991789854978, Name: "dirty", NotificationConf: &dirty}, nil)

	// 关键断言：不报错（不会拖垮 list 查询），conf 降级为 nil。
	assert.NoError(t, err)
	assert.NotNil(t, got)
	assert.Nil(t, got.NotificationConf)
	// 降级后运行期行为等价历史/未配置：默认飞书开启、webhook 关闭。
	assert.True(t, got.NotificationConf.GetNotificationConfOrDefault().Feishu.Enable)
	assert.False(t, got.NotificationConf.GetNotificationConfOrDefault().Webhook.Enable)
}

// 批量 list 场景：脏数据实验夹在正常实验中间，单条降级不影响其他实验与整体转换。
func TestExptConverter_NotificationConf_DirtyDoesNotBreakBatch(t *testing.T) {
	t.Parallel()

	conv := NewExptConverter()

	good, err := conv.DO2PO(&entity.Experiment{ID: 1, Name: "good", NotificationConf: sampleNotificationConf()})
	assert.NoError(t, err)
	dirtyBlob := []byte(`{"Webhook":{"Enable":true,"URLs":"efwefewfewf"}}`)
	pos := []*model.Experiment{
		good,
		{ID: 2, Name: "dirty", NotificationConf: &dirtyBlob},
		{ID: 3, Name: "history"}, // NULL conf
	}

	for _, po := range pos {
		do, convErr := conv.PO2DO(po, nil)
		assert.NoError(t, convErr)
		assert.NotNil(t, do)
	}

	// 正常实验的 conf 仍无损保留。
	goodDO, err := conv.PO2DO(pos[0], nil)
	assert.NoError(t, err)
	assert.Equal(t, sampleNotificationConf(), goodDO.NotificationConf)
	// 脏数据实验降级为 nil。
	dirtyDO, err := conv.PO2DO(pos[1], nil)
	assert.NoError(t, err)
	assert.Nil(t, dirtyDO.NotificationConf)
}

// 模板继承：notification_conf 经 template_conf BLOB 嵌套承载，JSON 往返无损，
// 派生实验时可从 ExptTemplateConfiguration 提取并拷入新实验。
func TestExptTemplateConfiguration_NotificationConf_NestedRoundtrip(t *testing.T) {
	t.Parallel()

	tmplConf := &entity.ExptTemplateConfiguration{
		NotificationConf: sampleNotificationConf(),
	}

	// 序列化进 template_conf BLOB 容器。
	blob, err := json.Marshal(tmplConf)
	assert.NoError(t, err)

	// 反序列化回，子段无损。
	var got entity.ExptTemplateConfiguration
	assert.NoError(t, json.Unmarshal(blob, &got))
	assert.Equal(t, tmplConf.NotificationConf, got.NotificationConf)

	// 模拟派生：把模板 conf 拷入新实验，再经 DB convertor 往返仍无损。
	conv := NewExptConverter()
	derived := &entity.Experiment{ID: 555, Name: "derived", NotificationConf: got.NotificationConf}
	po, err := conv.DO2PO(derived)
	assert.NoError(t, err)
	back, err := conv.PO2DO(po, nil)
	assert.NoError(t, err)
	assert.Equal(t, tmplConf.NotificationConf, back.NotificationConf)
}

// 历史模板 template_conf JSON 无 notification_conf 子段时反序列化为 nil（向后兼容）。
func TestExptTemplateConfiguration_NotificationConf_AbsentIsNil(t *testing.T) {
	t.Parallel()

	var got entity.ExptTemplateConfiguration
	assert.NoError(t, json.Unmarshal([]byte(`{"connector_conf":{}}`), &got))
	assert.Nil(t, got.NotificationConf)
}
