// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable complexity */
import { useLocation, useParams } from 'react-router-dom';
import { useEffect } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { type LanguageType } from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import { type Form } from '@coze-arch/coze-design';

import {
  CodeEvaluatorLanguageFE,
  codeEvaluatorLanguageMap,
  defaultTestData,
} from '@/constants';
import {
  TestDataSource,
  type IFormValues,
} from '@/components/evaluator-code/types';

interface UseTemplateQueryParams {
  formRef: React.RefObject<Form<IFormValues>>;
  setTemplateInfo: React.Dispatch<
    React.SetStateAction<{
      key: string;
      name: string;
      lang: string;
    } | null>
  >;
}

/**
 * Code 评估器 复制 & 模板场景
 * 监听query templateKey和templateLang参数, 更新form和setTemplateInfo
 * @param params
 */
const useTemplateQuery = (params: UseTemplateQueryParams) => {
  const { formRef, setTemplateInfo } = params;
  const location = useLocation();

  const { spaceID } = useSpace();
  const { id } = useParams<{ id: string }>();

  // 从URL查询参数中获取模板信息
  useEffect(() => {
    const searchParams = new URLSearchParams(location.search);
    const templateKey = searchParams.get('templateKey');
    const templateLang = searchParams.get('templateLang');
    // 复制评估器
    if (id) {
      StoneEvaluationApi.GetEvaluator({
        workspace_id: spaceID,
        evaluator_id: id,
      })
        .then(res => {
          const { evaluator } = res;
          if (evaluator) {
            const sourceName = res.evaluator?.name || '';
            const copySubfix = '_Copy';
            const newName = sourceName
              .slice(0, 50 - copySubfix.length)
              .concat(copySubfix);
            const { code_evaluator } =
              evaluator.current_version?.evaluator_content || {};
            formRef.current?.formApi.setValues({
              name: newName,
              description: evaluator.description,
              config: {
                funcExecutor: {
                  code: code_evaluator?.code_content || '',
                  language:
                    codeEvaluatorLanguageMap[
                      code_evaluator?.language_type as LanguageType
                    ] || CodeEvaluatorLanguageFE.Javascript,
                },
                testData: {
                  source: TestDataSource.Custom,
                  customData: defaultTestData[0],
                },
              },
            });
          }
        })
        .catch(err => console.error(err));
    } else if (templateKey) {
      // 获取模板信息
      StoneEvaluationApi.GetTemplateV2({
        evaluator_template_id: templateKey === 'custom' ? '0' : templateKey,
        custom_code: templateKey === 'custom' ? true : false,
      })
        .then(res => {
          const { evaluator_template } = res;
          const { code_evaluator } =
            evaluator_template?.evaluator_content || {};
          if (
            evaluator_template?.evaluator_content?.code_evaluator &&
            code_evaluator
          ) {
            setTemplateInfo({
              key: templateKey,
              name:
                evaluator_template?.name || I18n.t('evaluate_unnamed_template'),
              lang: templateLang || '',
            });
            const formApi = formRef.current?.formApi;
            if (!formApi) {
              return;
            }
            const funcExecutorValue = formApi.getValue('config.funcExecutor');
            const name = formApi.getValue('name');

            const extraPayload: Record<string, unknown> = {
              ...funcExecutorValue,
            };

            // 使用表单API更新表单值
            if (code_evaluator.lang_2_code_content && formApi) {
              extraPayload.code =
                code_evaluator?.lang_2_code_content?.[templateLang as string] ||
                '';
            }

            // 更新语言选择
            if (templateLang && formApi) {
              extraPayload.language =
                codeEvaluatorLanguageMap[templateLang as string];
            }
            if (!name) {
              formApi.setValue(
                'name',
                evaluator_template?.name || I18n.t('evaluate_unnamed_template'),
              );
            }

            formApi.setValue('config.funcExecutor', extraPayload);
          }
        })
        .catch(err => console.error(err));
    }
  }, [location.search]);
};

export { useTemplateQuery };
