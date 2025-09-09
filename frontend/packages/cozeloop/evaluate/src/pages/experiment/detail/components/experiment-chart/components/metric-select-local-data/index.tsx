// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useMemo, type ReactNode } from 'react';

import cls from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import {
  type ColumnAnnotation,
  type ColumnEvaluator,
} from '@cozeloop/api-schema/evaluation';
import { tag } from '@cozeloop/api-schema/data';
import { Select, type SelectProps } from '@coze-arch/coze-design';

import { EvaluatorInfo, AnnotationInfo } from '@/components/info-tag';

import styles from './index.module.less';

export interface MetricValueType {
  type: 'evaluator' | 'annotation';
  id: string;
}

interface OptionNode {
  label: ReactNode;
  value: string;
  evaluator?: ColumnEvaluator;
  annotation?: ColumnAnnotation;
}
interface Props extends Omit<SelectProps, 'value' | 'onChange'> {
  value?: MetricValueType[];
  onChange?: (value: MetricValueType[]) => void;
  evaluators?: ColumnEvaluator[];
  annotations?: ColumnAnnotation[];
}
export function MetricSelectLocalData({
  value,
  onChange,
  evaluators,
  annotations,
  className,
  ...props
}: Props) {
  const { options, valueMap } = useMemo(() => {
    const evaluatorOptions = (evaluators || []).map(e => ({
      label: (
        <LabelWrapper>
          <EvaluatorInfo evaluator={e} className="w-full overflow-hidden" />
        </LabelWrapper>
      ),
      value: e.evaluator_version_id ?? '',
      evaluator: e,
      type: 'evaluator',
    }));

    const annotationOptions = (annotations || [])
      .filter(a => a.content_type !== tag.TagContentType.FreeText)
      .map(a => ({
        label: (
          <LabelWrapper>
            <AnnotationInfo annotation={a} className="w-full overflow-hidden" />
          </LabelWrapper>
        ),
        value: a.tag_key_id ?? '',
        annotation: a,
        type: 'annotation',
      }));
    const finalOptions = [...evaluatorOptions, ...annotationOptions];
    return {
      options: finalOptions,
      valueMap: finalOptions.reduce(
        (pre, cur) => {
          pre[cur.value] = {
            type: cur.type as 'evaluator' | 'annotation',
            id: cur.value,
          };
          return pre;
        },
        {} as unknown as Record<string, MetricValueType>,
      ),
    };
  }, [evaluators, annotations]);

  return (
    <Select
      prefix={I18n.t('indicator')}
      placeholder={I18n.t('please_select', { field: '' })}
      value={value?.map(v => v.id)}
      onChange={v => {
        const newVal =
          (v as string[] | undefined)?.map(id => valueMap[id]) || [];
        onChange?.(newVal.filter(Boolean));
      }}
      {...props}
      className={cls(styles['render-selected-item'], className)}
      renderSelectedItem={optionNode => {
        const node = optionNode as OptionNode;
        return {
          isRenderInTag: true,
          content: (
            <LabelWrapper>
              {node.evaluator ? (
                <EvaluatorInfo
                  evaluator={node.evaluator}
                  className="text-xs"
                  tagProps={{ size: 'mini' }}
                />
              ) : (
                <AnnotationInfo
                  annotation={node.annotation}
                  className="text-xs"
                  tagProps={{ size: 'mini' }}
                />
              )}
            </LabelWrapper>
          ),
        };
      }}
      optionList={options}
    />
  );
}

function LabelWrapper(props: { children: ReactNode }) {
  return (
    <span className="ml-1 overflow-hidden text-xs" style={{ maxWidth: 160 }}>
      {props.children}
    </span>
  );
}
