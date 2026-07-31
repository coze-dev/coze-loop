// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"encoding/base64"
	"sort"
	"strconv"
	"strings"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

const (
	defaultSharedPageSize = 20
	maxSharedPageSize     = 200
)

func sharedPageTokenForPageNumber(pageNumber, pageSize *int32) (*string, error) {
	if pageNumber == nil || *pageNumber <= 1 {
		return nil, nil
	}
	size := defaultSharedPageSize
	if pageSize != nil {
		size = int(*pageSize)
	}
	if size <= 0 || size > maxSharedPageSize {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("invalid page_size"))
	}
	offset := (int(*pageNumber) - 1) * size
	return gptr.Of(base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(offset)))), nil
}

func paginateShared[T any](items []T, pageSize *int32, pageToken *string) ([]T, *string, bool, error) {
	size := defaultSharedPageSize
	if pageSize != nil {
		size = int(*pageSize)
	}
	if size <= 0 || size > maxSharedPageSize {
		return nil, nil, false, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("invalid page_size"))
	}

	offset := 0
	if pageToken != nil && strings.TrimSpace(*pageToken) != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(*pageToken))
		if err != nil {
			return nil, nil, false, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("invalid page_token"))
		}
		parsed, err := strconv.Atoi(string(decoded))
		if err != nil || parsed < 0 {
			return nil, nil, false, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("invalid page_token"))
		}
		offset = parsed
	}
	if offset >= len(items) {
		return []T{}, nil, false, nil
	}

	end := min(offset+size, len(items))
	hasMore := end < len(items)
	var nextPageToken *string
	if hasMore {
		nextPageToken = new(string)
		*nextPageToken = base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(end)))
	}
	return items[offset:end], nextPageToken, hasMore, nil
}

func paginateSharedAccessContexts(
	accessCtxs []*entity.ResourceAccessContext,
	resourceIDs []int64,
	pageSize *int32,
	pageToken *string,
) ([]*entity.ResourceAccessContext, int64, *string, bool, error) {
	sortedAccessCtxs := normalizeSharedAccessContexts(accessCtxs, resourceIDs)
	total := int64(len(sortedAccessCtxs))
	page, nextPageToken, hasMore, err := paginateShared(sortedAccessCtxs, pageSize, pageToken)
	return page, total, nextPageToken, hasMore, err
}

func normalizeSharedAccessContexts(
	accessCtxs []*entity.ResourceAccessContext,
	resourceIDs []int64,
) []*entity.ResourceAccessContext {
	var resourceIDSet map[int64]struct{}
	if len(resourceIDs) > 0 {
		resourceIDSet = make(map[int64]struct{}, len(resourceIDs))
		for _, resourceID := range resourceIDs {
			resourceIDSet[resourceID] = struct{}{}
		}
	}

	accessByKey := make(map[string]*entity.ResourceAccessContext, len(accessCtxs))
	for _, accessCtx := range accessCtxs {
		if accessCtx == nil || accessCtx.ResourceSpaceID <= 0 || accessCtx.ResourceID <= 0 {
			continue
		}
		if resourceIDSet != nil {
			if _, ok := resourceIDSet[accessCtx.ResourceID]; !ok {
				continue
			}
		}
		accessByKey[sharedResourceKey(accessCtx.ResourceSpaceID, accessCtx.ResourceID)] = accessCtx
	}

	sortedAccessCtxs := make([]*entity.ResourceAccessContext, 0, len(accessByKey))
	for _, accessCtx := range accessByKey {
		sortedAccessCtxs = append(sortedAccessCtxs, accessCtx)
	}
	sort.Slice(sortedAccessCtxs, func(i, j int) bool {
		if sortedAccessCtxs[i].ResourceSpaceID != sortedAccessCtxs[j].ResourceSpaceID {
			return sortedAccessCtxs[i].ResourceSpaceID < sortedAccessCtxs[j].ResourceSpaceID
		}
		return sortedAccessCtxs[i].ResourceID < sortedAccessCtxs[j].ResourceID
	})
	return sortedAccessCtxs
}

func batchGetSharedEvaluationSets(
	ctx context.Context,
	evaluationSetService service.IEvaluationSetService,
	callerSpaceID int64,
	accessCtxs []*entity.ResourceAccessContext,
) ([]*entity.EvaluationSet, error) {
	idsBySource := make(map[int64][]int64)
	for _, accessCtx := range accessCtxs {
		idsBySource[accessCtx.ResourceSpaceID] = append(idsBySource[accessCtx.ResourceSpaceID], accessCtx.ResourceID)
	}

	setByKey := make(map[string]*entity.EvaluationSet, len(accessCtxs))
	for sourceSpaceID, evaluationSetIDs := range idsBySource {
		sharedOption := &entity.SharedResourceOption{IsShared: true, SourceSpaceID: gptr.Of(sourceSpaceID)}
		sets, err := evaluationSetService.BatchGetEvaluationSets(
			ctx,
			gptr.Of(callerSpaceID),
			evaluationSetIDs,
			gptr.Of(false),
			sharedOption,
		)
		if err != nil {
			return nil, err
		}
		for _, set := range sets {
			if set != nil {
				setByKey[sharedResourceKey(sourceSpaceID, set.ID)] = set
			}
		}
	}

	sets := make([]*entity.EvaluationSet, 0, len(accessCtxs))
	for _, accessCtx := range accessCtxs {
		set := setByKey[sharedResourceKey(accessCtx.ResourceSpaceID, accessCtx.ResourceID)]
		if set == nil {
			continue
		}
		sharedInfo := accessCtx.SharedInfo()
		set.SharedInfo = sharedInfo
		if set.EvaluationSetVersion != nil {
			set.EvaluationSetVersion.SharedInfo = sharedInfo
		}
		if accessCtx.AccessLevel != entity.SharedAccessLevelReadable && set.EvaluationSetVersion != nil {
			set.EvaluationSetVersion.EvaluationSetSchema = nil
		}
		sets = append(sets, set)
	}
	return sets, nil
}
