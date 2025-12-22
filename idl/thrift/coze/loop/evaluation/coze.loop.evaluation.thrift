namespace go coze.loop.evaluation

include "coze.loop.evaluation.eval_set.thrift"
include "coze.loop.evaluation.evaluator.thrift"
include "coze.loop.evaluation.expt.thrift"
include "coze.loop.evaluation.eval_target.thrift"
include "coze.loop.evaluation.openapi.thrift"
include "coze.loop.evaluation.spi.thrift"
include "../trajectory.thrift"

typedef trajectory.Trajectory Trajectory

service EvaluationSetService extends coze.loop.evaluation.eval_set.EvaluationSetService{}

service EvaluatorService extends coze.loop.evaluation.evaluator.EvaluatorService{}

service ExperimentService extends coze.loop.evaluation.expt.ExperimentService{}

service EvalTargetService extends coze.loop.evaluation.eval_target.EvalTargetService{}

service EvalOpenAPIService extends coze.loop.evaluation.openapi.EvaluationOpenAPIService{}

service EvalSPIService extends coze.loop.evaluation.spi.EvaluationSPIService{}
