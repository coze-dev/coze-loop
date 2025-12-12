// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */
import { useCallback, useState } from 'react';

import { useLatest, useThrottleFn } from 'ahooks';
import { EVENT_NAMES, sendEvent } from '@cozeloop/tea-adapter';
import { I18n } from '@cozeloop/i18n-adapter';
import { ResizeSidesheet } from '@cozeloop/components';
import { useNavigateModule } from '@cozeloop/biz-hooks-adapter';
import {
  EvaluatorTagKey,
  EvaluatorTagType,
  type EvaluatorTemplate,
} from '@cozeloop/api-schema/evaluation';
import { IconCozDocument, IconCozPlus } from '@coze-arch/coze-design/icons';
import { Button, Divider, IconButton, Tooltip } from '@coze-arch/coze-design';

import { evaluatorFilterTransform } from './utils';
import { EvaluatorTypeTagText, type ListTemplatesParams } from './types';
import { EvaluatorTemplateList } from './evaluator-template-list';
import { EvaluatorTemplateDetail } from './evaluator-template-detail';
import { listTemplatesV2 } from './api';

export function EvaluatorTemplateListPanel({
  defaultEvaluatorType = EvaluatorTypeTagText.Prompt,
  disabledEvaluatorTypes,
  onClose,
  onApply,
  onClickCard,
}: {
  defaultEvaluatorType?: EvaluatorTypeTagText;
  disabledEvaluatorTypes?: EvaluatorTypeTagText[];
  onClose: () => void;
  onApply: (
    template: EvaluatorTemplate,
    /* 额外的业务配置 */
    options?: {
      /** 仅当评估器类型为代码评估器，并且用户选择了语言类型时，此参数才会生效 */
      codeLanguageType?: string;
    },
  ) => void;
  onClickCard?: (template: EvaluatorTemplate) => void;
}) {
  const navigateModule = useNavigateModule();
  const [activeTemplate, setActiveTemplate] = useState<EvaluatorTemplate>();
  const onApplyRef = useLatest(onApply);

  const listTemplates = useCallback(async (params: ListTemplatesParams) => {
    const res = await listTemplatesV2({
      ...params,
      filter_option: evaluatorFilterTransform(params),
    });
    const list = res?.evaluator_templates || [];
    return {
      list,
      total: Number(res?.total) || 0,
    };
  }, []);

  const { run: handleApply } = useThrottleFn(
    (template, options?: { codeLanguageType?: string }) => {
      onApplyRef.current?.(template, options);
    },
    { wait: 500 },
  );

  const getCardHeaderActions = useCallback(
    (template: EvaluatorTemplate) => (
      <>
        {template.evaluator_info?.user_manual_url ? (
          <>
            <Tooltip content={I18n.t('document')} theme="dark">
              <div>
                <IconButton
                  icon={<IconCozDocument />}
                  size="mini"
                  onClick={() => {
                    const url = template.evaluator_info?.user_manual_url;
                    window.open(url);
                  }}
                />
              </div>
            </Tooltip>
            <Divider layout="vertical" />
          </>
        ) : null}

        <Tooltip content={I18n.t('apply')} theme="dark">
          <div>
            <Button
              color="highlight"
              size="mini"
              onClick={() => {
                handleApply(template);
              }}
            >
              {I18n.t('apply')}
            </Button>
          </div>
        </Tooltip>
      </>
    ),
    [],
  );

  const handleOnClickCard = useCallback(
    (template: EvaluatorTemplate) => {
      setActiveTemplate(template);
      onClickCard?.(template);
    },
    [onClickCard],
  );

  const handleCardApply = useCallback(
    (template: EvaluatorTemplate, options?: { codeLanguageType?: string }) => {
      handleApply(template, options);
      sendEvent(EVENT_NAMES.cozeloop_evaluator_sample_apply, {
        apply_entry: 'evaluator_template_card',
      });
    },
    [handleApply],
  );

  const handleCardDetailApply = useCallback(
    (template: EvaluatorTemplate, options?: { codeLanguageType?: string }) => {
      handleApply(template, options);
      sendEvent(EVENT_NAMES.cozeloop_evaluator_sample_apply, {
        apply_entry: 'evaluator_template_card_detail',
      });
    },
    [handleApply],
  );

  return (
    <ResizeSidesheet
      title={I18n.t('evaluator_template')}
      width={1200}
      visible={true}
      bodyStyle={{ padding: 0, overflow: 'hidden' }}
      onCancel={onClose}
    >
      <EvaluatorTemplateList
        colCount={2}
        className={activeTemplate ? 'hidden' : ''}
        defaultEvaluatorType={defaultEvaluatorType}
        disabledEvaluatorTypes={disabledEvaluatorTypes}
        listHeaderAction={({ filters }) => {
          const evaluatorTypeTag = filters?.[EvaluatorTagKey.Category]?.[0];
          // 这个标签是和服务端约定的，没有 EvaluatorType 枚举
          const isCode = evaluatorTypeTag === 'Code';
          return (
            <Button
              icon={<IconCozPlus />}
              onClick={() => {
                if (isCode) {
                  navigateModule(
                    'evaluation/evaluators/create/code?templateKey=custom&templateLang=Python',
                  );
                } else {
                  navigateModule('evaluation/evaluators/create/llm');
                }
              }}
            >
              {I18n.t('custom_create_{type}_evaluator', {
                type: evaluatorTypeTag,
              })}
            </Button>
          );
        }}
        filterOptions={{
          showClearAll: false,
          showScenariosClear: true,
          isTypeSingleSelect: true,
          // 模板列表
          tagType: EvaluatorTagType.Template,
        }}
        getCardHeaderActions={getCardHeaderActions}
        listTemplates={listTemplates}
        onClickCard={handleOnClickCard}
        onApply={handleCardApply}
      />
      {activeTemplate ? (
        <EvaluatorTemplateDetail
          template={activeTemplate}
          className="h-full"
          onApply={handleCardDetailApply}
          onBack={() => {
            setActiveTemplate(undefined);
          }}
        />
      ) : null}
    </ResizeSidesheet>
  );
}
