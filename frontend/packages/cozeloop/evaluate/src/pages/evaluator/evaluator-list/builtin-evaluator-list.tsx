// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useCallback } from 'react';

import { EVENT_NAMES, sendEvent } from '@cozeloop/tea-adapter';
import { I18n } from '@cozeloop/i18n-adapter';
import { useNavigateModule } from '@cozeloop/biz-hooks-adapter';
import {
  EvaluatorTagType,
  type Evaluator,
  type EvaluatorTemplate,
} from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import { IconCozDocument } from '@coze-arch/coze-design/icons';
import { IconButton, Tooltip } from '@coze-arch/coze-design';

import { evaluatorFilterTransform } from '../evaluator-template/utils';
import { type ListTemplatesParams } from '../evaluator-template/types';
import { EvaluatorTemplateList } from '../evaluator-template/evaluator-template-list';

function EvaluatorToTemplate(evaluator: Evaluator): EvaluatorTemplate {
  const {
    workspace_id,
    evaluator_id,
    name,
    description,
    evaluator_type,
    current_version,
    evaluator_info,
    tags,
  } = evaluator;
  const { evaluator_content } = current_version || {};
  const template: EvaluatorTemplate & { current_version_id?: string } = {
    workspace_id,
    id: evaluator_id ?? '',
    name,
    description,
    evaluator_type,
    evaluator_content,
    evaluator_info,
    current_version_id: current_version?.id,
    tags,
  };
  return template;
}

export function BuiltinEvaluatorList() {
  const navigateModule = useNavigateModule();
  const listTemplates = useCallback(async (params: ListTemplatesParams) => {
    const res = await StoneEvaluationApi.ListEvaluators({
      ...params,
      builtin: true,
      filter_option: evaluatorFilterTransform(params),
    });
    const templates = res?.evaluators?.map(EvaluatorToTemplate);
    return {
      list: templates ?? [],
      total: Number(res?.total) || 0,
    };
  }, []);

  const getCardHeaderActions = useCallback((template: EvaluatorTemplate) => {
    const userManualUrl = template?.evaluator_info?.user_manual_url;
    if (!userManualUrl) {
      return null;
    }
    return (
      <Tooltip content={I18n.t('document')} theme="dark">
        <div className="cursor-pointer">
          <IconButton
            icon={<IconCozDocument />}
            size="mini"
            onClick={() => {
              window.open(userManualUrl);
            }}
          />
        </div>
      </Tooltip>
    );
  }, []);
  return (
    <EvaluatorTemplateList
      listTemplates={listTemplates}
      getCardHeaderActions={getCardHeaderActions}
      // 预置评估器列表
      filterOptions={{
        tagType: EvaluatorTagType.Evaluator,
      }}
      onClickCard={template => {
        sendEvent(EVENT_NAMES.cozeloop_pre_evaluator_detail_view, {
          pre_evaluator_card_name: template?.name || '',
        });
        navigateModule(
          // @ts-expect-error 临时处理 todo: @yangfeng.alanyf
          `evaluation/evaluators/${template.id}?isPreEvaluator=true&version=${template?.current_version_id || ''}`,
        );
      }}
    />
  );
}
