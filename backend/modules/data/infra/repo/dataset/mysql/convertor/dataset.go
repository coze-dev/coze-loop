// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"github.com/bytedance/sonic"

	"github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/entity"
	"github.com/coze-dev/coze-loop/backend/modules/data/infra/repo/dataset/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/errno"
)

func DatasetDO2PO(dataset *entity.Dataset) (po *model.Dataset, err error) {
	if dataset == nil {
		return nil, nil
	}
	po = &model.Dataset{
		ID:             dataset.ID,
		AppID:          dataset.AppID,
		SpaceID:        dataset.SpaceID,
		SchemaID:       dataset.SchemaID,
		Name:           dataset.Name,
		Description:    dataset.Description,
		Category:       string(dataset.Category),
		BizCategory:    dataset.BizCategory,
		Tag:            dataset.Tag,
		Status:         string(dataset.Status),
		SecurityLevel:  string(dataset.SecurityLevel),
		Visibility:     string(dataset.Visibility),
		LatestVersion:  dataset.LatestVersion,
		NextVersionNum: dataset.NextVersionNum,
		LastOperation:  string(dataset.LastOperation),
		CreatedBy:      dataset.CreatedBy,
		CreatedAt:      dataset.CreatedAt,
		UpdatedBy:      dataset.UpdatedBy,
		UpdatedAt:      dataset.UpdatedAt,
		ExpiredAt:      dataset.ExpiredAt,
	}
	if dataset.Features != nil {
		po.Features, err = sonic.Marshal(dataset.Features)
		if err != nil {
			return nil, errno.JSONErr(err, "marshal dataset.feature failed, data=%v", dataset.Features)
		}
	}
	if dataset.Spec != nil {
		po.Spec, err = sonic.Marshal(dataset.Spec)
		if err != nil {
			return nil, errno.JSONErr(err, "marshal dataset.spec failed, data=%v", dataset.Spec)
		}
	}
	return po, nil
}

func DatasetPO2DO(p *model.Dataset) (*entity.Dataset, error) {
	if p == nil {
		return nil, nil
	}
	m := &entity.Dataset{
		ID:             p.ID,
		AppID:          p.AppID,
		SpaceID:        p.SpaceID,
		SchemaID:       p.SchemaID,
		Name:           p.Name,
		Description:    p.Description,
		Category:       entity.DatasetCategory(p.Category),
		BizCategory:    p.BizCategory,
		Tag:            p.Tag,
		Status:         entity.DatasetStatus(p.Status),
		SecurityLevel:  entity.SecurityLevel(p.SecurityLevel),
		Visibility:     entity.DatasetVisibility(p.Visibility),
		Spec:           nil,
		Features:       nil,
		LatestVersion:  p.LatestVersion,
		NextVersionNum: p.NextVersionNum,
		LastOperation:  entity.DatasetOpType(p.LastOperation),
		CreatedBy:      p.CreatedBy,
		CreatedAt:      p.CreatedAt,
		UpdatedBy:      p.UpdatedBy,
		UpdatedAt:      p.UpdatedAt,
		ExpiredAt:      p.ExpiredAt,
	}
	if err := sonic.Unmarshal(p.Spec, &m.Spec); err != nil {
		return nil, errno.JSONErr(err, "unmarshal dataset.spec failed, data=%v", p.Spec)
	}
	if err := sonic.Unmarshal(p.Features, &m.Features); err != nil {
		return nil, errno.JSONErr(err, "unmarshal dataset.features failed, data=%v", p.Features)
	}
	return m, nil
}
