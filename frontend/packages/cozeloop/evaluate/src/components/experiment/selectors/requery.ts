// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
import {
  type EvalTarget,
  type EvalTargetVersion,
  type ListSourceEvalTargetsRequest,
  type ListSourceEvalTargetsResponse,
  type ListSourceEvalTargetVersionsRequest,
  type ListSourceEvalTargetVersionsResponse,
  type EvaluationSetVersion,
  type EvaluationSet,
  type ListEvaluationSetsRequest,
  type ListEvaluationSetsResponse,
  type ListEvaluationSetVersionsRequest,
  type ListEvaluationSetVersionsResponse,
} from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';

import { wait } from '@/utils/experiment';

function createId() {
  return `id_${Math.random().toFixed(10).slice(-6)}`;
}

const useMock = false;

export async function listSourceEvalTargetVersion(
  params: ListSourceEvalTargetVersionsRequest,
): Promise<ListSourceEvalTargetVersionsResponse> {
  if (!useMock) {
    return StoneEvaluationApi.ListSourceEvalTargetVersions(params);
  }
  await wait(300);
  const targets = new Array(Math.floor(Math.random() * 10 + 1))
    .fill(1)
    .map((_, index) => {
      const item: EvalTargetVersion = {
        id: createId(),
        source_target_version: `0.0.${index + 1}`,
        eval_target_content: {
          coze_bot: {
            bot_name: `CozeBot ${index + 1}`,
            description: I18n.t('version_description'),
          },
          prompt: {
            name: `Prompt ${index + 1}`,
          },
        },
      };
      return item;
    });
  return {
    versions: targets,
    next_page_token: '10',
  };
}

export async function listSourceEvalTarget(
  params: ListSourceEvalTargetsRequest,
): Promise<ListSourceEvalTargetsResponse> {
  if (!useMock) {
    return StoneEvaluationApi.ListSourceEvalTargets(params);
  }
  await wait(300);
  const targets = new Array(Math.floor(Math.random() * 10 + 3))
    .fill(1)
    .map((_, index) => {
      const item: EvalTarget = {
        id: createId(),
        eval_target_version: {
          id: createId(),
          eval_target_content: {
            coze_bot: {
              bot_name: `CozeBot ${params.name ?? ''} ${index + 1}`,
              description: I18n.t('this_is_a_cozebot'),
            },
            prompt: {
              name: `Prompt ${params.name ?? ''} ${index + 1}`,
            },
          },
        },
      };
      return item;
    });
  return {
    eval_targets: targets,
    next_page_token: '10',
  };
}

export async function listEvaluationSets(
  params: ListEvaluationSetsRequest,
): Promise<ListEvaluationSetsResponse> {
  if (!useMock) {
    return StoneEvaluationApi.ListEvaluationSets(params);
  }
  await wait(300);
  const targets = new Array(10).fill(1).map((_, index) => {
    const item: EvaluationSet = {
      id: createId(),
      name: I18n.t('pedia_dataset'),
      base_info: {
        created_at: new Date().toLocaleString(),
        created_by: {
          user_id: 'xxx',
          name: I18n.t('user_zhangsan'),
        },
      },
      evaluation_set_version: {
        version: '0.0.1',
      },
    };
    return item;
  });
  return {
    evaluation_sets: targets,
    next_page_token: '34',
  };
}

export async function listEvaluationSetVersions(
  params: ListEvaluationSetVersionsRequest,
): Promise<ListEvaluationSetVersionsResponse> {
  if (!useMock) {
    return StoneEvaluationApi.ListEvaluationSetVersions(params);
  }
  await wait(300);
  const list: EvaluationSetVersion[] = new Array(
    Math.floor(Math.random() * 10 + 1),
  )
    .fill(1)
    .map((_, index) => {
      const item: EvaluationSetVersion = {
        id: createId(),
        version: `0.0.${index + 1}`,
        description: I18n.t('version_description'),
      };
      return item;
    });
  return {
    versions: list,
    next_page_token: '34',
  };
}
