import classNames from 'classnames';
import { JumpIconButton } from '@cozeloop/components';
import { useResourcePageJump } from '@cozeloop/biz-hooks-adapter';
import {
  type ColumnAnnotation,
  type AnnotateRecord,
} from '@cozeloop/api-schema/evaluation';
import { Divider, Popover, Tooltip } from '@coze-arch/coze-design';

import { TypographyText } from '../text-ellipsis';
import { TagRender } from './tag/tag-render';

interface NameScoreTagProps {
  name?: string;
  annotation?: ColumnAnnotation;
  annotateRecord?: AnnotateRecord;
  tagID?: Int64;
  enableLinkJump?: boolean;
  defaultShowAction?: boolean;
  border?: boolean;
}

export function AnnotationNameScoreTag({
  name,
  annotation,
  annotateRecord,
  tagID,
  enableLinkJump,
  defaultShowAction = false,
  border = true,
}: NameScoreTagProps) {
  const { getTagDetailURL } = useResourcePageJump();

  const borderClass = border
    ? 'border border-solid border-[var(--coz-stroke-primary)] cursor-pointer hover:bg-[var(--coz-mg-primary)] hover:border-[var(--coz-stroke-plus)]'
    : '';
  return (
    <div className={'group flex items-center text-[var(--coz-fg-primary)]'}>
      <div
        className={`flex items-center h-5 px-2 rounded-[3px] gap-1 text-xs font-medium ${borderClass}`}
      >
        <TypographyText className="max-w-10">{name ?? '-'}</TypographyText>
        <Divider layout="vertical" style={{ height: 12 }} />
        {annotation ? (
          <TagRender
            className="!max-w-[100px] overflow-hidden"
            annotation={annotation}
            annotateRecord={annotateRecord}
          />
        ) : null}
      </div>
      <div className={classNames('flex items-center', 'ml-1')}>
        {enableLinkJump ? (
          <Tooltip theme="dark" content="查看标签详情">
            <div className="flex items-center">
              <JumpIconButton
                className={defaultShowAction ? '' : 'hidden group-hover:flex'}
                onClick={() => {
                  window.open(getTagDetailURL(tagID || ''));
                }}
              />
            </div>
          </Tooltip>
        ) : null}
      </div>
    </div>
  );
}

export function AnnotationNameScore({
  annotation,
  annotationResult,
  enablePopover = false,
  border = true,
  defaultShowAction,
}: {
  annotation?: ColumnAnnotation;
  annotationResult?: AnnotateRecord;
  enablePopover?: boolean;
  border?: boolean;
  defaultShowAction?: boolean;
}) {
  if (!enablePopover) {
    return (
      <AnnotationNameScoreTag
        name={annotation?.tag_key_name}
        annotation={annotation}
        annotateRecord={annotationResult}
        tagID={annotation?.tag_key_id}
        enableLinkJump={true}
        defaultShowAction={defaultShowAction}
        border={border}
      />
    );
  }
  return (
    <Popover
      position="top"
      trigger="click"
      stopPropagation
      content={
        <div className="p-1" style={{ color: 'var(--coz-fg-secondary)' }}>
          <AnnotationNameScoreTag
            name={annotation?.tag_key_name}
            annotation={annotation}
            annotateRecord={annotationResult}
            tagID={annotation?.tag_key_id}
            enableLinkJump={true}
            defaultShowAction={true}
            border={false}
          />
        </div>
      }
    >
      <div>
        <AnnotationNameScoreTag
          name={annotation?.tag_key_name}
          annotation={annotation}
          annotateRecord={annotationResult}
          tagID={annotation?.tag_key_id}
          border={border}
          defaultShowAction={defaultShowAction}
        />
      </div>
    </Popover>
  );
}
