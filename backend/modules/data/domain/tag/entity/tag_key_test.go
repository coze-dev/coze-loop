// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/tag"
	"github.com/coze-dev/coze-loop/backend/modules/data/infra/repo/tag/mysql/gorm_gen/model"
)

func TestTagKey_ToPO(t *testing.T) {
	buf, _ := sonic.Marshal(nil)
	tests := []struct {
		name    string
		req     *TagKey
		want    *model.TagKey
		wantErr bool
	}{
		{
			name:    "nil",
			req:     nil,
			want:    nil,
			wantErr: false,
		},
		{
			name: "normal case",
			req: &TagKey{
				ID:            123,
				TagTargetType: []TagTargetType{TagTargetTypeObserve, TagTargetTypeEvaluation},
			},
			want: &model.TagKey{
				ID:            123,
				TagTargetType: "observe,evaluation",
				Version:       "",
				Spec:          buf,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := tt.req.ToPO()
			if (err != nil) != tt.wantErr {
				t.Errorf("ToPO() error = %v, wantErr %v", err, tt.wantErr)
			}
			if res == nil {
				assert.Equal(t, tt.want, res)
			} else {
				assert.Equal(t, tt.want.ID, res.ID)
				assert.Equal(t, tt.want.TagTargetType, res.TagTargetType)
				assert.Equal(t, tt.want.Version, res.Version)
			}
		})
	}
}

func TestTagKey_ToTagInfoDTO(t *testing.T) {
	tests := []struct {
		name string
		req  *TagKey
		want *tag.TagInfo
	}{
		{
			name: "nil",
			req:  nil,
			want: nil,
		},
		{
			name: "normal case",
			req: &TagKey{
				ID:            123,
				TagTargetType: []TagTargetType{TagTargetTypeObserve, TagTargetTypeEvaluation},
			},
			want: &tag.TagInfo{
				ID: gptr.Of(int64(123)),
				DomainTypeList: []tag.TagDomainType{
					tag.TagDomainTypeObserve,
					tag.TagDomainTypeEvaluation,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := tt.req.ToTagInfoDTO()
			if res == nil {
				assert.Equal(t, tt.want, res)
			} else {
				assert.Equal(t, tt.want.ID, res.ID)
				assert.Equal(t, tt.want.DomainTypeList, res.DomainTypeList)
				assert.Equal(t, tt.want.Version, res.Version)
			}
		})
	}
}

func TestTagKey_SetVersionNum(t *testing.T) {
	tests := []struct {
		name       string
		req        *TagKey
		versionNum int32
	}{
		{
			name:       "nil",
			req:        nil,
			versionNum: 123,
		},
		{
			name: "normal case",
			req: &TagKey{
				TagValues: []*TagValue{
					{},
					{},
				},
			},
			versionNum: 123,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.req.SetVersionNum(tt.versionNum)
			if tt.req == nil {
			} else {
				assert.Equal(t, gptr.Of(tt.versionNum), tt.req.VersionNum)
				for _, v := range tt.req.TagValues {
					assert.Equal(t, gptr.Of(tt.versionNum), v.VersionNum)
				}
			}
		})
	}
}

func TestTagKey_SetSpaceID(t *testing.T) {
	tests := []struct {
		name    string
		req     *TagKey
		spaceID int64
	}{
		{
			name:    "nil",
			req:     nil,
			spaceID: 123,
		},
		{
			name: "normal case",
			req: &TagKey{
				TagValues: []*TagValue{
					{},
					{},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.req.SetSpaceID(tt.spaceID)
			if tt.req == nil {
			} else {
				assert.Equal(t, tt.spaceID, tt.req.SpaceID)
				for _, v := range tt.req.TagValues {
					assert.Equal(t, tt.spaceID, v.SpaceID)
				}
			}
		})
	}
}

func TestTagKey_SetAppID(t *testing.T) {
	tests := []struct {
		name  string
		req   *TagKey
		appID int32
	}{
		{
			name:  "nil",
			req:   nil,
			appID: 123,
		},
		{
			name: "normal case",
			req: &TagKey{
				TagValues: []*TagValue{
					{},
					{},
				},
			},
			appID: 123,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.req.SetAppID(tt.appID)
			if tt.req == nil {
			} else {
				assert.Equal(t, tt.appID, tt.req.AppID)
				for _, v := range tt.req.TagValues {
					assert.Equal(t, tt.appID, v.AppID)
				}
			}
		})
	}
}

func TestTagKey_SetCreatedBy(t *testing.T) {
	tests := []struct {
		name      string
		req       *TagKey
		createdBy string
	}{
		{
			name:      "nil",
			req:       nil,
			createdBy: "123",
		},
		{
			name: "normal case",
			req: &TagKey{
				TagValues: []*TagValue{
					{},
					{},
				},
			},
			createdBy: "123",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.req.SetCreatedBy(tt.createdBy)
			if tt.req == nil {
			} else {
				assert.Equal(t, gptr.Of(tt.createdBy), tt.req.CreatedBy)
				for _, v := range tt.req.TagValues {
					assert.Equal(t, gptr.Of(tt.createdBy), v.CreatedBy)
				}
			}
		})
	}
}

func TestTagKey_SetUpdatedBy(t *testing.T) {
	tests := []struct {
		name      string
		req       *TagKey
		updatedBy string
	}{
		{
			name:      "nil",
			req:       nil,
			updatedBy: "123",
		},
		{
			name: "normal case",
			req: &TagKey{
				TagValues: []*TagValue{
					{},
					{},
				},
			},
			updatedBy: "123",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.req.SetUpdatedBy(tt.updatedBy)
			if tt.req == nil {
			} else {
				assert.Equal(t, gptr.Of(tt.updatedBy), tt.req.UpdatedBy)
				for _, v := range tt.req.TagValues {
					assert.Equal(t, gptr.Of(tt.updatedBy), v.UpdatedBy)
				}
			}
		})
	}
}

func TestTagKey_SetCreatedAt(t *testing.T) {
	ts := time.Now()
	tests := []struct {
		name      string
		req       *TagKey
		createdAt time.Time
	}{
		{
			name:      "nil",
			req:       nil,
			createdAt: ts,
		},
		{
			name: "normal case",
			req: &TagKey{
				TagValues: []*TagValue{
					{},
					{},
				},
			},
			createdAt: ts,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.req.SetCreatedAt(tt.createdAt)
			if tt.req == nil {
			} else {
				assert.Equal(t, tt.createdAt, tt.req.CreatedAt)
				for _, v := range tt.req.TagValues {
					assert.Equal(t, tt.createdAt, v.CreatedAt)
				}
			}
		})
	}
}

func TestTagKey_SetUpdateAt(t *testing.T) {
	ts := time.Now()
	tests := []struct {
		name      string
		req       *TagKey
		updatedAt time.Time
	}{
		{
			name:      "nil",
			req:       nil,
			updatedAt: ts,
		},
		{
			name: "normal case",
			req: &TagKey{
				TagValues: []*TagValue{
					{},
					{},
				},
			},
			updatedAt: ts,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.req.SetUpdatedAt(tt.updatedAt)
			if tt.req == nil {
			} else {
				assert.Equal(t, tt.updatedAt, tt.req.UpdatedAt)
				for _, v := range tt.req.TagValues {
					assert.Equal(t, tt.updatedAt, v.UpdatedAt)
				}
			}
		})
	}
}

func TestTagKey_SetStatus(t *testing.T) {
	tests := []struct {
		name   string
		req    *TagKey
		status TagStatus
	}{
		{
			name:   "nil",
			req:    nil,
			status: TagStatusActive,
		},
		{
			name: "normal case",
			req: &TagKey{
				TagValues: []*TagValue{
					{},
					{},
				},
			},
			status: TagStatusActive,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.req.SetStatus(tt.status)
			if tt.req == nil {
			} else {
				assert.Equal(t, tt.status, tt.req.Status)
				for _, v := range tt.req.TagValues {
					assert.Equal(t, tt.status, v.Status)
				}
			}
		})
	}
}

func TestTagKey_SetTagKeyID(t *testing.T) {
	tests := []struct {
		name     string
		req      *TagKey
		tagKeyID int64
	}{
		{
			name:     "nil",
			req:      nil,
			tagKeyID: 123,
		},
		{
			name: "normal case",
			req: &TagKey{
				TagValues: []*TagValue{
					{},
					{},
				},
			},
			tagKeyID: 123,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.req.SetTagKeyID(tt.tagKeyID)
			if tt.req == nil {
			} else {
				assert.Equal(t, tt.tagKeyID, tt.req.TagKeyID)
				for _, v := range tt.req.TagValues {
					assert.Equal(t, tt.tagKeyID, v.TagKeyID)
				}
			}
		})
	}
}

func TestTagKey_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     *TagKey
		spec    *TagSpec
		wantErr bool
	}{
		{
			name:    "nil",
			req:     nil,
			spec:    nil,
			wantErr: true,
		},
		{
			name:    "tag type is undefined",
			req:     &TagKey{},
			spec:    nil,
			wantErr: true,
		},
		{
			name: "tag content type is undefined",
			req: &TagKey{
				TagType: TagTypeTag,
				Status:  TagStatusUndefined,
			},
			spec:    nil,
			wantErr: true,
		},
		{
			name: "free text tag values is more than 0",
			req: &TagKey{
				TagType: TagTypeTag,
				TagValues: []*TagValue{
					{},
				},
				TagContentType: TagContentTypeFreeText,
			},
			wantErr: true,
		},
		{
			name: "boolean tag values is not equal 2",
			req: &TagKey{
				TagType: TagTypeTag,
				TagValues: []*TagValue{
					{},
				},
				TagContentType: TagContentTypeBoolean,
			},
			wantErr: true,
		},
		{
			name: "tag key name length is more than 50",
			req: &TagKey{
				TagType:    TagTypeTag,
				TagKeyName: "123123123123123123123123123123123123123123123123123123123123123123123123",
				TagValues: []*TagValue{
					{},
				},
				TagContentType: TagContentTypeCategorical,
			},
			wantErr: true,
		},
		{
			name: "tag key name is empty",
			req: &TagKey{
				TagKeyName: "",
				TagType:    TagTypeTag,
				TagValues: []*TagValue{
					{},
				},
				TagContentType: TagContentTypeCategorical,
			},
			wantErr: true,
		},
		{
			name: "description is more than 200",
			req: &TagKey{
				TagKeyName: "123",
				TagType:    TagTypeTag,
				TagValues: []*TagValue{
					{},
				},
				Description:    gptr.Of("123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123123"),
				TagContentType: TagContentTypeCategorical,
			},
			wantErr: true,
		},
		{
			name: "normal case",
			req: &TagKey{
				TagKeyName:     "123",
				TagType:        TagTypeTag,
				TagValues:      []*TagValue{},
				TagContentType: TagContentTypeCategorical,
			},
			spec:    &TagSpec{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate(tt.spec)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTagKey_SplitTagValues(t *testing.T) {
	tests := []struct {
		name   string
		key    *TagKey
		oldNum int
		newNum int
	}{
		{
			name:   "nil",
			oldNum: 0,
			newNum: 0,
		},
		{
			name:   "normal case",
			oldNum: 4,
			newNum: 4,
			key: &TagKey{TagValues: []*TagValue{
				{
					TagValueID: 123,
				},
				{
					Children: []*TagValue{
						{},
						{},
						{},
					},
				},
				{
					TagValueID: 321,
					Children: []*TagValue{
						{
							TagValueID: 3456,
						},
						{
							TagValueID: 345,
						},
					},
				},
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old, new1 := tt.key.SplitTagValues()
			assert.Equal(t, len(old), tt.oldNum)
			assert.Equal(t, len(new1), tt.newNum)
		})
	}
}

func TestTagKey_CalculateChangeLogs(t *testing.T) {
	tests := []struct {
		name      string
		sourceKey *TagKey
		targetKey *TagKey
		want      []*ChangeLog
		wantErr   bool
	}{
		{
			name:    "source key is nil",
			wantErr: true,
		},
		{
			name:      "normal case----pre tag key is nil",
			targetKey: &TagKey{},
			sourceKey: nil,
			want: []*ChangeLog{
				{
					ChangeTarget: TagChangeTargetTypeTag,
					Operation:    TagOperationTypeCreate,
				},
			},
		},
		{
			name: "normal case----update",
			sourceKey: &TagKey{
				TagType:        TagTypeTag,
				Status:         TagStatusInactive,
				TagKeyName:     "123",
				TagContentType: TagContentTypeCategorical,
				TagValues: []*TagValue{
					{
						TagKeyID:     123,
						TagValueName: "234",
					},
				},
			},
			targetKey: &TagKey{
				TagType:        TagTypeOption,
				Status:         TagStatusActive,
				TagKeyName:     "321",
				Description:    gptr.Of("123"),
				TagContentType: TagContentTypeBoolean,
				TagValues: []*TagValue{
					{
						TagValueName: "123",
					},
				},
			},
			want: []*ChangeLog{
				{
					ChangeTarget: TagChangeTargetTypeTagType,
					Operation:    TagOperationTypeUpdate,
					BeforeValue:  string(TagTypeTag),
					AfterValue:   string(TagTypeOption),
				},
				{
					ChangeTarget: TagChangeTargetTypeTagStatus,
					Operation:    TagOperationTypeUpdate,
					BeforeValue:  string(TagStatusInactive),
					AfterValue:   string(TagStatusActive),
				},
				{
					ChangeTarget: TagChangeTargetTypeTagName,
					Operation:    TagOperationTypeUpdate,
					BeforeValue:  "123",
					AfterValue:   "321",
				},
				{
					ChangeTarget: TagChangeTargetTypeTagDescription,
					Operation:    TagOperationTypeUpdate,
					BeforeValue:  "",
					AfterValue:   "123",
				},
				{
					ChangeTarget: TagChangeTargetTypeTagContentType,
					Operation:    TagOperationTypeUpdate,
					BeforeValue:  string(TagContentTypeCategorical),
					AfterValue:   string(TagContentTypeBoolean),
				},
				{
					ChangeTarget: TagChangeTargetTypeTagValueName,
					Operation:    TagOperationTypeCreate,
					AfterValue:   "123",
					TargetValue:  "123",
				},
			},
			wantErr: false,
		},
		{
			name: "delete tag values",
			targetKey: &TagKey{
				TagValues: []*TagValue{},
			},
			sourceKey: &TagKey{
				TagValues: []*TagValue{
					{
						TagValueID:   123,
						TagValueName: "123",
					},
				},
			},
			want: []*ChangeLog{
				{
					ChangeTarget: TagChangeTargetTypeTagValueName,
					Operation:    TagOperationTypeDelete,
					BeforeValue:  "123",
					TargetValue:  "123",
				},
			},
		},
		{
			name: "update tag value name",
			targetKey: &TagKey{
				TagValues: []*TagValue{
					{
						TagValueID:   123,
						TagValueName: "234",
					},
				},
			},
			sourceKey: &TagKey{
				TagValues: []*TagValue{
					{
						TagValueID:   123,
						TagValueName: "123",
					},
				},
			},
			want: []*ChangeLog{
				{
					ChangeTarget: TagChangeTargetTypeTagValueName,
					Operation:    TagOperationTypeUpdate,
					BeforeValue:  "123",
					TargetValue:  "234",
					AfterValue:   "234",
				},
			},
		},
		{
			name: "update tag value status",
			targetKey: &TagKey{
				TagValues: []*TagValue{
					{
						TagValueID:   123,
						TagValueName: "123",
						Status:       TagStatusInactive,
					},
				},
			},
			sourceKey: &TagKey{
				TagValues: []*TagValue{
					{
						TagValueID:   123,
						TagValueName: "123",
						Status:       TagStatusActive,
					},
				},
			},
			want: []*ChangeLog{
				{
					ChangeTarget: TagChangeTargetTypeTagValueStatus,
					Operation:    TagOperationTypeUpdate,
					BeforeValue:  string(TagStatusActive),
					TargetValue:  "123",
					AfterValue:   string(TagStatusInactive),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := tt.targetKey.CalculateChangeLogs(tt.sourceKey)
			if (err != nil) != tt.wantErr {
				t.Errorf("CalculateChangeLogs() error = %v, wantErr %v", err, tt.wantErr)
			}
			assert.Equal(t, len(res), len(tt.want))
			for i := 0; i < len(res); i++ {
				assert.Equal(t, res[i], tt.want[i])
			}
		})
	}
}
