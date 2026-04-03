package apis

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"code.byted.org/flowdevops/cozeloop-gen-commercial/kitex_gen/coze/loop/observability/saas/insight/insightsaasservice"
)

var localInsightSvc insightsaasservice.Client

// CreateInsightTask .
// @router /api/observability/v1/insight/tasks [POST]
func CreateInsightTask(ctx context.Context, c *app.RequestContext) {
	invokeAndRender(ctx, c, localInsightSvc.CreateInsightTask)
}

// GetInsightTask .
// @router /api/observability/v1/insight/tasks/:task_id [GET]
func GetInsightTask(ctx context.Context, c *app.RequestContext) {
	invokeAndRender(ctx, c, localInsightSvc.GetInsightTask)
}

// ListInsightTasks .
// @router /api/observability/v1/insight/tasks [GET]
func ListInsightTasks(ctx context.Context, c *app.RequestContext) {
	invokeAndRender(ctx, c, localInsightSvc.ListInsightTasks)
}

// UpdateInsightTask .
// @router /api/observability/v1/insight/tasks/:task_id [PATCH]
func UpdateInsightTask(ctx context.Context, c *app.RequestContext) {
	invokeAndRender(ctx, c, localInsightSvc.UpdateInsightTask)
}

// DeleteInsightTask .
// @router /api/observability/v1/insight/tasks/:task_id [DELETE]
func DeleteInsightTask(ctx context.Context, c *app.RequestContext) {
	invokeAndRender(ctx, c, localInsightSvc.DeleteInsightTask)
}

// GetInsightTaskRun .
// @router /api/observability/v1/insight/task_runs/:task_run_id [GET]
func GetInsightTaskRun(ctx context.Context, c *app.RequestContext) {
	invokeAndRender(ctx, c, localInsightSvc.GetInsightTaskRun)
}

// ListInsightTaskRuns .
// @router /api/observability/v1/insight/tasks/:task_id/runs [GET]
func ListInsightTaskRuns(ctx context.Context, c *app.RequestContext) {
	invokeAndRender(ctx, c, localInsightSvc.ListInsightTaskRuns)
}

// RetryInsightTaskRun .
// @router /api/observability/v1/insight/task_runs/:task_run_id/retry [POST]
func RetryInsightTaskRun(ctx context.Context, c *app.RequestContext) {
	invokeAndRender(ctx, c, localInsightSvc.RetryInsightTaskRun)
}

// GetInsightReport .
// @router /api/observability/v1/insight/reports/:report_id [GET]
func GetInsightReport(ctx context.Context, c *app.RequestContext) {
	invokeAndRender(ctx, c, localInsightSvc.GetInsightReport)
}
