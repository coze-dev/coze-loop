// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type ReactNode } from 'react';

import classNames from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import { TypographyText } from '@cozeloop/evaluate-components';
import {
  type ColumnAnnotation,
  type ColumnEvaluator,
} from '@cozeloop/api-schema/evaluation';
import { Tag, type TagProps } from '@coze-arch/coze-design';

export function BaseInfo(props: {
  name: ReactNode;
  tag: ReactNode;
  className?: string;
  tagProps?: TagProps;
}) {
  const { name, tag, tagProps } = props;

  return (
    <div
      className={classNames(
        'group inline-flex items-center gap-1 max-w-[100%]',
        props.className,
      )}
    >
      <TypographyText>{name ?? '-'}</TypographyText>
      {tag ? (
        <Tag
          size="small"
          color="primary"
          {...tagProps}
          className={classNames('shrink-0 font-normal', tagProps?.className)}
        >
          {tag}
        </Tag>
      ) : null}
    </div>
  );
}

export function AnnotationInfo(props: {
  annotation?: ColumnAnnotation;
  className?: string;
  tagProps?: TagProps;
}) {
  const { annotation, tagProps, className } = props;
  if (!annotation) {
    return null;
  }
  return (
    <BaseInfo
      name={annotation.tag_key_name}
      tag={I18n.t('manual_calibration')}
      tagProps={tagProps}
      className={className}
    />
  );
}

export function EvaluatorInfo(props: {
  evaluator?: ColumnEvaluator;
  className?: string;
  tagProps?: TagProps;
}) {
  const { evaluator, tagProps } = props;
  if (!evaluator) {
    return null;
  }
  return (
    <BaseInfo
      name={evaluator.name}
      tag={evaluator.version}
      tagProps={tagProps}
    />
  );
}
