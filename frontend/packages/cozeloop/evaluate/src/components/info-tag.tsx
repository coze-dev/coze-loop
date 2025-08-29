import { type ReactNode } from 'react';

import classNames from 'classnames';
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
      tag="人工标注"
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
