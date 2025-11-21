namespace go coze.loop.observability.task

include "../../../base.thrift"
include "./domain/filter.thrift"
include "./domain/task.thrift"
include "./domain/common.thrift"

struct CreateTaskRequest {
    1: required task.Task task (api.body = "task"),
    2: optional common.Session session (api.body="session"),

    255: optional base.Base base,
}

struct CreateTaskResponse {
    1: optional i64 task_id (api.js_conv="true" api.body="task_id"),

    255: optional base.BaseResp BaseResp
}

struct UpdateTaskRequest {
    1: required i64 task_id (api.js_conv="true" api.path="task_id"),
    2: required i64 workspace_id (api.js_conv='true', go.tag='json:"workspace_id"', api.body="workspace_id", vt.gt="0")
    3: optional task.TaskStatus task_status (api.body = "task_status"),
    4: optional string description  (api.body = "description"),
    5: optional task.EffectiveTime effective_time (api.body = "effective_time"),
    6: optional double sample_rate (api.body = "sample_rate"),
    7: optional common.Session session (api.body="session"),

    255: optional base.Base base,
}

struct UpdateTaskResponse {
    255: optional base.BaseResp BaseResp
}

struct ListTasksRequest {
    1: required i64 workspace_id (api.js_conv='true', go.tag='json:"workspace_id"', api.body="workspace_id", vt.gt="0")
    2: optional filter.TaskFilterFields task_filters (api.body = "task_filters"),

    101: optional i32 limit (api.body = "limit")   /* default 20 max 200 */
    102: optional i32 offset (api.body = "offset")
    103: optional common.OrderBy order_by (api.body = "order_by")
    255: optional base.Base base,
}

struct ListTasksResponse {
    1: optional list<task.Task> tasks (api.body = "tasks"),

    100: optional i64 total (api.js_conv="true" api.body="total"),
    255: optional base.BaseResp BaseResp
}

struct GetTaskRequest {
    1: required i64 task_id (api.path = "task_id" api.js_conv="true"),
    2: required i64 workspace_id (api.js_conv='true', go.tag='json:"workspace_id"', api.query="workspace_id", vt.gt="0")

    255: optional base.Base base,
}

struct GetTaskResponse {
    1: optional task.Task task (api.body="task"),

    255: optional base.BaseResp BaseResp
}

struct CheckTaskNameRequest {
    1: required i64 workspace_id (api.js_conv='true', go.tag='json:"workspace_id"', api.body="workspace_id", vt.gt="0")
    2: required string name                 (api.body='name')
    255: optional base.Base Base
}

struct CheckTaskNameResponse {
    1: optional bool Pass (agw.key='pass')
    2: optional string Message (agw.key='message')
    255: base.BaseResp BaseResp
}

service TaskService {
    CheckTaskNameResponse CheckTaskName(1: CheckTaskNameRequest req) (api.post = '/api/observability/v1/tasks/check_name')
    CreateTaskResponse CreateTask(1: CreateTaskRequest req) (api.post = '/api/observability/v1/tasks')
    UpdateTaskResponse UpdateTask(1: UpdateTaskRequest req) (api.put = '/api/observability/v1/tasks/:task_id')
    ListTasksResponse ListTasks(1: ListTasksRequest req) (api.post = '/api/observability/v1/tasks/list')
    GetTaskResponse GetTask(1: GetTaskRequest req) (api.get = '/api/observability/v1/tasks/:task_id')
}