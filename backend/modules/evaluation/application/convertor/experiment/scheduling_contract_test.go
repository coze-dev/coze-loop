// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	evaluatordto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	dexpt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestSchedulingContract_RequestFieldsAndReadPath(t *testing.T) {
	mode := dexpt.ExptSchedulingModeCentral
	priority := int32(7)
	req := &expt.CreateExperimentRequest{
		SchedulingMode: &mode,
		Priority:       &priority,
		Ext:            map[string]string{entity.SchedulingOwnerEnvExtKey: "ppe_contract"},
	}
	assertFieldType(t, reflect.TypeOf(*req), "SchedulingMode", "scheduling_mode")
	assertFieldType(t, reflect.TypeOf(*req), "Priority", "priority")

	conf, err := NewEvalConfConvert().ConvertToEntity(req, map[int64]*evaluatordto.EvaluatorRunConfig{})
	require.NoError(t, err)
	require.NotNil(t, conf.Scheduling)
	assert.Equal(t, entity.SchedulingModeCentral, conf.Scheduling.Mode)
	assert.Equal(t, int32(7), conf.Scheduling.Priority)
	assert.Equal(t, "ppe_contract", conf.Scheduling.OwnerEnv)

	submitMode := dexpt.ExptSchedulingModeCentral
	submitPriority := int32(5)
	submit := &expt.SubmitExperimentRequest{SchedulingMode: &submitMode, Priority: &submitPriority}
	assertFieldType(t, reflect.TypeOf(*submit), "SchedulingMode", "scheduling_mode")
	assert.Equal(t, dexpt.ExptSchedulingModeCentral, submit.GetSchedulingMode())
	assert.Equal(t, int32(5), submit.GetPriority())

	domainMode := dexpt.ExptSchedulingModeCentral
	domainPriority := int32(9)
	dto := &dexpt.Experiment{SchedulingMode: &domainMode, Priority: &domainPriority}
	assertFieldType(t, reflect.TypeOf(*dto), "SchedulingMode", "scheduling_mode")
	assert.Equal(t, dexpt.ExptSchedulingModeCentral, dto.GetSchedulingMode())
	assert.Equal(t, int32(9), dto.GetPriority())
}

func assertFieldType(t *testing.T, typ reflect.Type, goName, thriftName string) {
	t.Helper()
	field, ok := typ.FieldByName(goName)
	require.Truef(t, ok, "generated type %s missing field %s", typ.Name(), goName)
	assert.Truef(t, strings.HasPrefix(field.Tag.Get("thrift"), thriftName+","), "generated thrift tag mismatch: %q", field.Tag.Get("thrift"))
}
