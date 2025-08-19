namespace go coze.loop.apis

include "../foundation/coze.loop.foundation.auth.thrift"
include "../foundation/coze.loop.foundation.authn.thrift"
include "../foundation/coze.loop.foundation.user.thrift"
include "../foundation/coze.loop.foundation.space.thrift"
include "../foundation/coze.loop.foundation.file.thrift"
include "../foundation/coze.loop.foundation.openapi.thrift"
include "../evaluation/coze.loop.evaluation.eval_set.thrift"
include "../evaluation/coze.loop.evaluation.evaluator.thrift"
include "../evaluation/coze.loop.evaluation.eval_target.thrift"
include "../evaluation/coze.loop.evaluation.expt.thrift"
include "../data/coze.loop.data.dataset.thrift"
include "../prompt/coze.loop.prompt.manage.thrift"
include "../prompt/coze.loop.prompt.debug.thrift"
include "../prompt/coze.loop.prompt.execute.thrift"
include "../prompt/coze.loop.prompt.openapi.thrift"
include "../llm/coze.loop.llm.runtime.thrift"
include "../llm/coze.loop.llm.manage.thrift"
include "../observability/coze.loop.observability.trace.thrift"
include "../data/coze.loop.data.tag.thrift"
include "../observability/coze.loop.observability.openapi.thrift"

service EvaluationSetService extends coze.loop.evaluation.eval_set.EvaluationSetService{}
service EvaluatorService extends coze.loop.evaluation.evaluator.EvaluatorService{}
service EvalTargetService extends coze.loop.evaluation.eval_target.EvalTargetService{}
service ExperimentService extends coze.loop.evaluation.expt.ExperimentService{}

service DatasetService extends coze.loop.data.dataset.DatasetService{}
service TagService extends coze.loop.data.tag.TagService{}

service PromptManageService extends coze.loop.prompt.manage.PromptManageService{}
service PromptDebugService extends coze.loop.prompt.debug.PromptDebugService{}
service PromptExecuteService extends coze.loop.prompt.execute.PromptExecuteService{}
service PromptOpenAPIService extends coze.loop.prompt.openapi.PromptOpenAPIService{}

service LLMManageService extends coze.loop.llm.manage.LLMManageService {}
service LLMRuntimeService extends coze.loop.llm.runtime.LLMRuntimeService {}
service ObservabilityTraceService extends coze.loop.observability.trace.TraceService{}
service ObservabilityOpenAPIService extends coze.loop.observability.openapi.OpenAPIService{}

service FoundationAuthService extends coze.loop.foundation.auth.AuthService{}
service FoundationAuthNService extends coze.loop.foundation.authn.AuthNService{}
service FoundationUserService extends coze.loop.foundation.user.UserService{}
service FoundationSpaceService extends coze.loop.foundation.space.SpaceService{}
service FoundationFileService extends coze.loop.foundation.file.FileService{}
service FoundationOpenAPIService extends coze.loop.foundation.openapi.FoundationOpenAPIService{}
