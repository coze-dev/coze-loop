// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { annotation as AnnotationType } from '@cozeloop/api-schema/observation';
import { Divider, Typography } from '@coze-arch/coze-design';

interface ManualAnnotationProps {
  annotation: AnnotationType.Annotation;
}

export const ManualAnnotation = (props: ManualAnnotationProps) => {
  const { annotation } = props;
  const { type } = annotation;

  return (
    <div className="flex items-center text-[var(--coz-fg-primary)] max-w-full w-full">
      <div className="flex items-center h-5 px-2 rounded-[3px] gap-1 text-xs font-medium border border-solid border-[var(--coz-stroke-primary)] cursor-pointer max-w-full min-w-0">
        <Typography.Text
          ellipsis={{ showTooltip: true }}
          className="!text-[12px] !font-medium !text-[var(--coz-fg-primary)] !leading-[22px] !max-w-[100px]"
        >
          {type === AnnotationType.AnnotationType.ManualFeedback
            ? (annotation.manual_feedback?.tag_key_name ?? '-')
            : (annotation.key ?? '-')}
        </Typography.Text>
        <Divider layout="vertical" style={{ height: 12 }} />
        <Typography.Text
          ellipsis={{ showTooltip: true }}
          className="!text-[12px] !font-medium !text-[var(--coz-fg-primary)] !leading-[22px] overflow-hidden !max-w-[100px] flex-1"
        >
          {type === AnnotationType.AnnotationType.ManualFeedback
            ? (annotation.manual_feedback?.tag_value ?? '-')
            : (annotation.value ?? '-')}{' '}
        </Typography.Text>
      </div>
    </div>
  );
};
