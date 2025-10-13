// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"github.com/samber/lo"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/conv"
)

func NewQuotaSpaceExptConverter() *QuotaSpaceExptConverter {
	return &QuotaSpaceExptConverter{}
}

type QuotaSpaceExptConverter struct{}

func (QuotaSpaceExptConverter) FromDO(qse *entity.QuotaSpaceExpt) ([]byte, error) {
	bytes, err := json.Marshal(qse)
	if err != nil {
		return nil, errorx.Wrapf(err, "QuotaSpaceExpt json marshal failed")
	}
	return bytes, nil
}

func (QuotaSpaceExptConverter) ToDO(b []byte) (*entity.QuotaSpaceExpt, error) {
	qse := &entity.QuotaSpaceExpt{}
	bytes := toBytes(b)
	if err := lo.TernaryF(
		len(bytes) > 0,
		func() error { return json.Unmarshal(bytes, qse) },
		func() error { return nil },
	); err != nil {
		return nil, errorx.Wrapf(err, "QuotaSpaceExpt json unmarshal failed")
	}
	return qse, nil
}

// toBytes
//
//nolint:staticcheck
func toBytes(v any) []byte {
	switch v.(type) {
	case string:
		return conv.UnsafeStringToBytes(v.(string))
	case []byte:
		return v.([]byte)
	default:
		return nil
	}
}
