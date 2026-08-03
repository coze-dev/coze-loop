// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluation_set

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestSharedResourceInfoDO2DTOConverters(t *testing.T) {
	assert.Nil(t, SharedResourceInfoDO2DTO(nil))
	assert.Nil(t, OpenAPISharedResourceInfoDO2DTO(nil))

	info := &entity.SharedResourceInfo{
		IsShared:      true,
		SourceSpaceID: 100,
		AccessLevel:   entity.SharedAccessLevelExecute,
		VersionPolicy: entity.SharedVersionPolicyLatest,
	}
	dto := SharedResourceInfoDO2DTO(info)
	require.NotNil(t, dto)
	assert.True(t, dto.GetIsShared())
	assert.Equal(t, int64(100), dto.GetSourceSpaceID())
	assert.Equal(t, entity.SharedAccessLevelExecute, dto.GetAccessLevel())
	assert.Equal(t, entity.SharedVersionPolicyLatest, dto.GetVersionPolicy())

	openAPIDTO := OpenAPISharedResourceInfoDO2DTO(info)
	require.NotNil(t, openAPIDTO)
	assert.True(t, openAPIDTO.GetIsShared())
	assert.Equal(t, int64(100), openAPIDTO.GetSourceSpaceID())
	assert.Equal(t, entity.SharedAccessLevelExecute, openAPIDTO.GetAccessLevel())
	assert.Equal(t, entity.SharedVersionPolicyLatest, openAPIDTO.GetVersionPolicy())
}
