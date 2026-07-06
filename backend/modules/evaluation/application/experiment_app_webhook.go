// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
)

func (e *experimentApplication) ListWebhookDelivery(ctx context.Context, req *expt.ListWebhookDeliveryRequest) (r *expt.ListWebhookDeliveryResponse, err error) {
	return nil, errors.New("ListWebhookDelivery not implemented")
}
