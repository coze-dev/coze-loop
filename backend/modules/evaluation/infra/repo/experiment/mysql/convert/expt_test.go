// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

func TestExptConverter_DO2PO_MinimalFields(t *testing.T) {
	t.Parallel()

	exp := &entity.Experiment{
		ID:                 100,
		SpaceID:            1,
		Name:               "expt",
		ExperimentGroupKey: "grp",
		TargetType:         entity.EvalTargetTypeCozeBot,
		Status:             entity.ExptStatus_Pending,
		StatusMessage:      "queued",
		EvalSetSourceType:  entity.ExptEvalSetSourceType(1),
	}

	po, err := NewExptConverter().DO2PO(exp)
	assert.NoError(t, err)
	if !assert.NotNil(t, po) {
		return
	}
	assert.Equal(t, int64(100), po.ID)
	assert.Equal(t, "expt", po.Name)
	assert.Equal(t, "grp", po.ExperimentGroupKey)
	assert.Equal(t, int64(entity.EvalTargetTypeCozeBot), po.TargetType)
	assert.Equal(t, int32(entity.ExptStatus_Pending), po.Status)
	if assert.NotNil(t, po.StatusMessage) {
		assert.Equal(t, "queued", string(*po.StatusMessage))
	}
	assert.Equal(t, int32(1), po.EvalSetSourceType)
	// optional fields default to nil when zero
	assert.Nil(t, po.MaxAliveTime)
	assert.Nil(t, po.EvalConf)
	assert.Nil(t, po.TrialRunItemCount)
	assert.Nil(t, po.NotificationConf)
	assert.Equal(t, int64(0), po.ExptTemplateID)
}

func TestExptConverter_DO2PO_FullFields(t *testing.T) {
	t.Parallel()

	notif := &entity.ExptNotificationConf{}
	exp := &entity.Experiment{
		ID:                100,
		SpaceID:           1,
		MaxAliveTime:      3600,
		EvalConf:          &entity.EvaluationConfiguration{},
		TrialRunItemCount: 5,
		NotificationConf:  notif,
		ExptTemplateMeta:  &entity.ExptTemplateMeta{ID: 42},
	}

	po, err := NewExptConverter().DO2PO(exp)
	assert.NoError(t, err)
	if !assert.NotNil(t, po) {
		return
	}
	if assert.NotNil(t, po.MaxAliveTime) {
		assert.Equal(t, int64(3600), *po.MaxAliveTime)
	}
	if assert.NotNil(t, po.TrialRunItemCount) {
		assert.Equal(t, int64(5), *po.TrialRunItemCount)
	}
	if assert.NotNil(t, po.EvalConf) {
		var got entity.EvaluationConfiguration
		assert.NoError(t, json.Unmarshal(*po.EvalConf, &got))
	}
	if assert.NotNil(t, po.NotificationConf) {
		var got entity.ExptNotificationConf
		assert.NoError(t, json.Unmarshal(*po.NotificationConf, &got))
	}
	assert.Equal(t, int64(42), po.ExptTemplateID)
}

func TestExptConverter_PO2DO_FillsGroupKeyFromID(t *testing.T) {
	t.Parallel()

	po := &model.Experiment{
		ID:                 200,
		SpaceID:            1,
		Name:               "expt",
		ExperimentGroupKey: "",
		Status:             int32(entity.ExptStatus_Success),
	}

	do, err := NewExptConverter().PO2DO(po, nil)
	assert.NoError(t, err)
	if !assert.NotNil(t, do) {
		return
	}
	// Empty group key falls back to str(ID)
	assert.Equal(t, "200", do.ExperimentGroupKey)
	assert.Equal(t, entity.ExptStatus_Success, do.Status)
	assert.Empty(t, do.EvaluatorVersionRef)
	assert.Nil(t, do.ExptTemplateMeta)
	assert.Nil(t, do.NotificationConf)
}

func TestExptConverter_PO2DO_KeepsExplicitGroupKey(t *testing.T) {
	t.Parallel()

	po := &model.Experiment{
		ID:                 200,
		ExperimentGroupKey: "explicit-group",
	}
	do, err := NewExptConverter().PO2DO(po, nil)
	assert.NoError(t, err)
	assert.Equal(t, "explicit-group", do.ExperimentGroupKey)
}

func TestExptConverter_PO2DO_EvaluatorRefs(t *testing.T) {
	t.Parallel()

	po := &model.Experiment{ID: 200}
	refs := []*model.ExptEvaluatorRef{
		{EvaluatorVersionID: 1, EvaluatorID: 10},
		{EvaluatorVersionID: 2, EvaluatorID: 20},
	}
	do, err := NewExptConverter().PO2DO(po, refs)
	assert.NoError(t, err)
	if assert.Len(t, do.EvaluatorVersionRef, 2) {
		assert.Equal(t, int64(1), do.EvaluatorVersionRef[0].EvaluatorVersionID)
		assert.Equal(t, int64(10), do.EvaluatorVersionRef[0].EvaluatorID)
	}
}

func TestExptConverter_PO2DO_DecodesEvalConf(t *testing.T) {
	t.Parallel()

	blob, err := json.Marshal(&entity.EvaluationConfiguration{})
	assert.NoError(t, err)

	po := &model.Experiment{
		ID:       200,
		EvalConf: &blob,
	}
	do, err := NewExptConverter().PO2DO(po, nil)
	assert.NoError(t, err)
	assert.NotNil(t, do.EvalConf)
}

func TestExptConverter_PO2DO_FailsOnMalformedEvalConf(t *testing.T) {
	t.Parallel()

	bad := []byte("{not-json")
	po := &model.Experiment{ID: 200, EvalConf: &bad}
	do, err := NewExptConverter().PO2DO(po, nil)
	assert.Nil(t, do)
	assert.Error(t, err)
}

func TestExptConverter_PO2DO_DecodesNotificationConf(t *testing.T) {
	t.Parallel()

	blob, err := json.Marshal(&entity.ExptNotificationConf{})
	assert.NoError(t, err)

	po := &model.Experiment{ID: 200, NotificationConf: &blob}
	do, err := NewExptConverter().PO2DO(po, nil)
	assert.NoError(t, err)
	assert.NotNil(t, do.NotificationConf)
}

func TestExptConverter_PO2DO_FailsOnMalformedNotificationConf(t *testing.T) {
	t.Parallel()

	bad := []byte("{not-json")
	po := &model.Experiment{ID: 200, NotificationConf: &bad}
	do, err := NewExptConverter().PO2DO(po, nil)
	assert.Nil(t, do)
	assert.Error(t, err)
}

func TestExptConverter_PO2DO_TemplateAndOptionalFields(t *testing.T) {
	t.Parallel()

	po := &model.Experiment{
		ID:                200,
		ExptTemplateID:    77,
		MaxAliveTime:      gptr.Of(int64(1200)),
		TrialRunItemCount: gptr.Of(int64(3)),
	}
	do, err := NewExptConverter().PO2DO(po, nil)
	assert.NoError(t, err)
	if assert.NotNil(t, do.ExptTemplateMeta) {
		assert.Equal(t, int64(77), do.ExptTemplateMeta.ID)
	}
	assert.Equal(t, int64(1200), do.MaxAliveTime)
	assert.Equal(t, int64(3), do.TrialRunItemCount)
}
