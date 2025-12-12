import * as common from './domain/common';
export { common };
import * as task from './domain/task';
export { task };
import * as filter from './domain/filter';
export { filter };
import * as base from './../../../base';
export { base };
import { createAPI } from './../../config';
export interface CreateTaskRequest {
  task: task.Task,
  session?: common.Session,
}
export interface CreateTaskResponse {
  task_id?: string
}
export interface UpdateTaskRequest {
  task_id: string,
  workspace_id: string,
  task_status?: task.TaskStatus,
  description?: string,
  effective_time?: task.EffectiveTime,
  sample_rate?: number,
  session?: common.Session,
}
export interface UpdateTaskResponse {}
export interface ListTasksRequest {
  workspace_id: string,
  task_filters?: filter.TaskFilterFields,
  /** default 20 max 200 */
  limit?: number,
  offset?: number,
  order_by?: common.OrderBy,
}
export interface ListTasksResponse {
  tasks?: task.Task[],
  total?: string,
}
export interface GetTaskRequest {
  task_id: string,
  workspace_id: string,
}
export interface GetTaskResponse {
  task?: task.Task
}
export interface CheckTaskNameRequest {
  workspace_id: string,
  name: string,
}
export interface CheckTaskNameResponse {
  pass?: boolean,
  message?: string,
}
export const CheckTaskName = /*#__PURE__*/createAPI<CheckTaskNameRequest, CheckTaskNameResponse>({
  "url": "/api/observability/v1/tasks/check_name",
  "method": "POST",
  "name": "CheckTaskName",
  "reqType": "CheckTaskNameRequest",
  "reqMapping": {
    "body": ["workspace_id", "name"]
  },
  "resType": "CheckTaskNameResponse",
  "schemaRoot": "api://schemas/observability_coze.loop.observability.task",
  "service": "observabilityTask"
});
export const CreateTask = /*#__PURE__*/createAPI<CreateTaskRequest, CreateTaskResponse>({
  "url": "/api/observability/v1/tasks",
  "method": "POST",
  "name": "CreateTask",
  "reqType": "CreateTaskRequest",
  "reqMapping": {
    "body": ["task", "session"]
  },
  "resType": "CreateTaskResponse",
  "schemaRoot": "api://schemas/observability_coze.loop.observability.task",
  "service": "observabilityTask"
});
export const UpdateTask = /*#__PURE__*/createAPI<UpdateTaskRequest, UpdateTaskResponse>({
  "url": "/api/observability/v1/tasks/:task_id",
  "method": "PUT",
  "name": "UpdateTask",
  "reqType": "UpdateTaskRequest",
  "reqMapping": {
    "path": ["task_id"],
    "body": ["workspace_id", "task_status", "description", "effective_time", "sample_rate", "session"]
  },
  "resType": "UpdateTaskResponse",
  "schemaRoot": "api://schemas/observability_coze.loop.observability.task",
  "service": "observabilityTask"
});
export const ListTasks = /*#__PURE__*/createAPI<ListTasksRequest, ListTasksResponse>({
  "url": "/api/observability/v1/tasks/list",
  "method": "POST",
  "name": "ListTasks",
  "reqType": "ListTasksRequest",
  "reqMapping": {
    "body": ["workspace_id", "task_filters", "limit", "offset", "order_by"]
  },
  "resType": "ListTasksResponse",
  "schemaRoot": "api://schemas/observability_coze.loop.observability.task",
  "service": "observabilityTask"
});
export const GetTask = /*#__PURE__*/createAPI<GetTaskRequest, GetTaskResponse>({
  "url": "/api/observability/v1/tasks/:task_id",
  "method": "GET",
  "name": "GetTask",
  "reqType": "GetTaskRequest",
  "reqMapping": {
    "path": ["task_id"],
    "query": ["workspace_id"]
  },
  "resType": "GetTaskResponse",
  "schemaRoot": "api://schemas/observability_coze.loop.observability.task",
  "service": "observabilityTask"
});