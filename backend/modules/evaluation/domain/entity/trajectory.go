// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"context"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/trajectory"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/conv"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type Trajectory trajectory.Trajectory

func (t *Trajectory) IsValid() bool {
	if t == nil || t.ID == nil || t.RootStep == nil {
		return false
	}
	return true
}

func (t *Trajectory) ToContent(ctx context.Context) *Content {
	if t == nil {
		return nil
	}

	cnt := &Content{
		ContentType: gptr.Of(ContentTypeText),
		Format:      gptr.Of(FieldDisplayFormat_JSON),
	}
	bytes, err := json.Marshal(t)
	if err != nil {
		logs.CtxError(ctx, "Trajectory json marshal fail, err: %s", err.Error())
		return cnt
	}

	str := conv.UnsafeBytesToString(bytes)
	cnt.Text = &str
	return cnt
}
