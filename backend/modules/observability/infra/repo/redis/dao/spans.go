// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package dao

import (
	"context"
	"errors"

	"github.com/coze-dev/coze-loop/backend/infra/redis"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	redis2 "github.com/redis/go-redis/v9"
)

const (
	keySpanID             = "span_id"
	keyPreviousResponseID = "previous_response_id"
)

//go:generate mockgen -destination=mocks/spans_dao.go -package=mocks . ISpansRedisDao
type ISpansRedisDao interface {
	GetPreSpans(ctx context.Context, respID string) (spanIDs, responseIDs []string, err error)
}

func NewSpansRedisDaoImpl(r redis.PersistentCmdable) (ISpansRedisDao, error) {
	return &SpansRedisDaoImpl{
		r: r,
	}, nil
}

type SpansRedisDaoImpl struct {
	r redis.PersistentCmdable
}

func (s SpansRedisDaoImpl) GetPreSpans(ctx context.Context, respID string) (spanIDs, responseIDs []string, err error) {
	preSpanIDs := make([]string, 0, 8)
	respIDByOrder := make([]string, 0, 8)
	preRespID := respID
	spanNum := 0
	spanNumLimit := int32(100)
	for preRespID != "" {
		rawVal, err := s.r.Get(ctx, preRespID).Result()
		if errors.Is(err, redis2.Nil) { // break chain, just end
			break
		}
		if err != nil {
			return nil, nil, errorx.WrapByCode(err, obErrorx.CommercialCommonInternalErrorCodeCode)
		}
		redisValue := make(map[string]string)
		if err = json.Unmarshal([]byte(rawVal), &redisValue); err != nil {
			return nil, nil, errorx.WrapByCode(err, obErrorx.CommercialCommonInternalErrorCodeCode)
		}
		spanID, ok := redisValue[keySpanID]
		if ok {
			preSpanIDs = append(preSpanIDs, spanID) // do not need order, only for select from db
		}
		respIDByOrder = append([]string{preRespID}, respIDByOrder...) // need order, for order SpanList
		preRespID, _ = redisValue[keyPreviousResponseID]              //nolint:staticcheck

		spanNum++
		if spanNum >= int(spanNumLimit) {
			break
		}
	}

	return preSpanIDs, respIDByOrder, nil
}
