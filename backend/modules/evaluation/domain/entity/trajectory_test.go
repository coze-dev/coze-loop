package entity

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	kitextrajectory "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/trajectory"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

func TestTrajectory_IsValid(t *testing.T) {
	var nilTraj *Trajectory
	assert.False(t, nilTraj.IsValid())

	t1 := &Trajectory{}
	assert.False(t, t1.IsValid())

	id := "trace-id"
	t2 := &Trajectory{
		ID: &id,
	}
	assert.False(t, t2.IsValid())

	t3 := &Trajectory{
		ID:       &id,
		RootStep: &kitextrajectory.RootStep{},
	}
	assert.True(t, t3.IsValid())
}

func TestTrajectory_ToContent_Nil(t *testing.T) {
	var nilTraj *Trajectory
	content := nilTraj.ToContent(context.Background())
	assert.Nil(t, content)
}

func TestTrajectory_ToContent_Normal(t *testing.T) {
	id := "trace-id"
	tr := &Trajectory{
		ID:       &id,
		RootStep: &kitextrajectory.RootStep{},
	}

	content := tr.ToContent(context.Background())
	if assert.NotNil(t, content) {
		if assert.NotNil(t, content.ContentType) {
			assert.Equal(t, ContentTypeText, *content.ContentType)
		}
		if assert.NotNil(t, content.Format) {
			assert.Equal(t, FieldDisplayFormat_JSON, *content.Format)
		}
		if assert.NotNil(t, content.Text) {
			var got Trajectory
			err := json.Unmarshal([]byte(*content.Text), &got)
			assert.NoError(t, err)
			assert.Equal(t, *tr, got)
		}
	}
}

