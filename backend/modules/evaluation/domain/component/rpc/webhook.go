package rpc

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

type IWebhookDispatcher interface {
	Dispatch(ctx context.Context, event *entity.ExptLifecycleEvent, expt *entity.Experiment) error
}
