/* eslint-disable complexity */
import { useState } from 'react';

import { useRequest } from 'ahooks';
import {
  getUrlParam,
  useEvalTargetDefinition,
} from '@cozeloop/evaluate-components';

import type { CreateExperimentValues } from '@/types/experiment/experiment-create';
import { batchGetExperiment } from '@/request/experiment';
import { getEvaluationSetVersion } from '@/request/evaluation-set';

import {
  experimentToCreateExperimentValues,
  evaluationSetToCreateExperimentValues,
} from '../tools';

export interface UseInitialDataOptions {
  spaceID: string;
  copyExperimentID?: string;
  evaluationSetID?: string;
  evaluationSetVersionID?: string;
  setValue: (value: CreateExperimentValues) => void;
}

export const useInitialData = ({
  spaceID,
  copyExperimentID,
  evaluationSetID,
  evaluationSetVersionID,
  setValue,
}: UseInitialDataOptions) => {
  const [initValue, setInitValue] = useState<CreateExperimentValues>({
    workspace_id: spaceID,
  } satisfies CreateExperimentValues);
  const { getEvalTargetDefinition, getEvalTargetDefinitionList } =
    useEvalTargetDefinition();
  const initialSource = getUrlParam('from');

  // 加载数据
  const { loading } = useRequest(
    async () => {
      // 复制实验
      if (copyExperimentID) {
        const res = await batchGetExperiment({
          workspace_id: spaceID,
          expt_ids: [copyExperimentID],
        });
        const experiment = res.experiments?.[0];

        if (experiment) {
          const data = experimentToCreateExperimentValues({
            experiment,
            spaceID,
          });

          let payload = {
            ...data,
            name: `${experiment.name}_copy`,
          };

          // 插件自定义处理数据
          const targetDefinition = getEvalTargetDefinition(
            data.evalTargetType as string,
          );
          if (targetDefinition?.transformCopyExperimentValues) {
            const copyData =
              await targetDefinition?.transformCopyExperimentValues?.(payload);
            if (copyData) {
              payload = { ...payload, ...copyData };
            }
          }

          setValue(payload);
          setInitValue(payload);
        }
      } else if (evaluationSetID && evaluationSetVersionID) {
        // 从评测集创建
        const { evaluation_set, version } = await getEvaluationSetVersion({
          workspace_id: spaceID,
          evaluation_set_id: evaluationSetID,
          version_id: evaluationSetVersionID,
        });

        if (evaluation_set && version) {
          const data = evaluationSetToCreateExperimentValues(
            evaluation_set,
            version,
            spaceID,
          );
          setValue(data);
          setInitValue(data);
        }
      } else if (initialSource) {
        // 评测对象三方来源初始化
        const source = initialSource.split('_')[1];
        const defsList = getEvalTargetDefinitionList();
        const targetDefinition = defsList.find(
          item => item.evalTargetSource === source,
        );

        if (targetDefinition && targetDefinition.getInitData) {
          const payload = await targetDefinition.getInitData?.(spaceID);
          setValue(payload);
          setInitValue(payload);
        }
      }
    },
    {
      refreshDeps: [copyExperimentID, evaluationSetID, evaluationSetVersionID],
    },
  );

  return {
    loading,
    initValue,
  };
};
