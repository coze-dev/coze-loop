// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"context"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/trajectory"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
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

func (t *Trajectory) FromContent(cnt *Content) error {
	if t == nil {
		return nil
	}

	if gptr.Indirect(cnt.ContentType) != ContentTypeText {
		return errorx.New("invalid trajectory content type: %v", cnt.ContentType)
	}

	if !gptr.Indirect(cnt.ContentOmitted) {
		if cnt.Text == nil || len(*(cnt.Text)) == 0 {
			return errorx.New("trajectory parse error: null content")
		}
		trj := &Trajectory{}
		if err := json.Unmarshal(conv.UnsafeStringToBytes(*cnt.Text), cnt); err != nil {
			return errorx.New("trajectory json unmarshal fail, raw: %v, err: %s", *cnt.Text, err.Error())
		}
		*t = *trj
		return nil
	}

	// todo(@liushengyang): ContentOmitted

	return nil
}
