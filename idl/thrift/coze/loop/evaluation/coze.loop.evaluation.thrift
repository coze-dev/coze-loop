namespace go coze.loop.evaluation

include "coze.loop.evaluation.eval_set.thrift"
include "coze.loop.evaluation.evaluator.thrift"
include "coze.loop.evaluation.expt.thrift"
include "coze.loop.evaluation.eval_target.thrift"

service EvaluationSetService extends coze.loop.evaluation.eval_set.EvaluationSetService{}

service EvaluatorService extends coze.loop.evaluation.evaluator.EvaluatorService{}

service ExperimentService extends coze.loop.evaluation.expt.ExperimentService{}

service EvalTargetService extends coze.loop.evaluation.eval_target.EvalTargetService{}