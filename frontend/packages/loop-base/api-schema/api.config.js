// apps/logistics/api.config.js

const path = require('path');

const config = [
  {
    // idl 根目录
    idlRoot: path.resolve(
      __dirname,
      'node_modules/.cache/coze-loop-idl/thrift/coze/loop',
    ),
    genMock: false,
    entries: {
      promptDebug: './prompt/coze.loop.prompt.debug.thrift',
      promptManage: './prompt/coze.loop.prompt.manage.thrift',
      observabilityTrace:
        './observability/coze.loop.observability.trace.thrift',
      observabilityTask: './observability/coze.loop.observability.task.thrift',
      observabilityMetric:
        './observability/coze.loop.observability.metric.thrift',
      evaluationEvalSet: './evaluation/coze.loop.evaluation.eval_set.thrift',
      evaluationEvalTarget:
        './evaluation/coze.loop.evaluation.eval_target.thrift',
      evaluationEvaluator: './evaluation/coze.loop.evaluation.evaluator.thrift',
      evaluationExpt: './evaluation/coze.loop.evaluation.expt.thrift',
      dataDataset: './data/coze.loop.data.dataset.thrift',
      dateTag: './data/coze.loop.data.tag.thrift',
      llmManage: './llm/coze.loop.llm.manage.thrift',
      foundationUpload: './foundation/coze.loop.foundation.file.thrift',
      foundationUser: './foundation/coze.loop.foundation.user.thrift',
      foundationAuthn: './foundation/coze.loop.foundation.authn.thrift',
      foundationSpace: './foundation/coze.loop.foundation.space.thrift',
    },
    commonCodePath: path.resolve(__dirname, './src/api/config.ts'), // 自定义配置文件
    output: './src/api/idl', // 产物所在位置
    plugins: [], // 自定义插件
  },
];

module.exports = config;
