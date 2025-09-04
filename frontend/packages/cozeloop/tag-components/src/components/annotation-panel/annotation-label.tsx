/* eslint-disable @typescript-eslint/no-explicit-any */
import { useState } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import { TooltipWhenDisabled } from '@cozeloop/components';
import { type tag } from '@cozeloop/api-schema/data';
import { withField } from '@coze-arch/coze-design';

import { TagSelect } from '../tag-select';
import { TagItem } from './label-item';
import { useAnnotationPanelContext } from './annotation-panel-context.ts';
import { type CreateAnnotationFormValues } from './annotation-content';

import styles from './index.module.less';
interface AnnotationLabelProps {
  field: string;
  tagItem?: CreateAnnotationFormValues['tags'][number];
  onCreateAnnotationSuccess?: (v?: string, annotationId?: string) => void;
  onTagSelectChange?: (v: any, tagInfo?: tag.TagInfo) => void;
  trace_id?: string;
  span_id?: string;
  start_time?: string;
  disableSelectList?: string[];
  isTagDisabled?: boolean;
}

const FormTagSelect = withField(TagSelect);
export const AnnotationLabel = (props: AnnotationLabelProps) => {
  const {
    field,
    onTagSelectChange,
    tagItem,
    onCreateAnnotationSuccess,
    trace_id,
    span_id,
    isTagDisabled,
    start_time,
    disableSelectList,
  } = props;
  const [loading, setLoading] = useState(false);
  const { setSaveLoading } = useAnnotationPanelContext();

  return (
    <div className={styles['annotation-label']}>
      <TooltipWhenDisabled
        content={I18n.t('tag_disabled_no_modification')}
        disabled={isTagDisabled}
        theme="dark"
      >
        <FormTagSelect
          style={{
            paddingTop: '6px',
            paddingBottom: '6px',
          }}
          field={`${field}.annotation.manual_feedback.template_tag_key_id`}
          placeholder={I18n.t('enter_tag_name')}
          noLabel
          className="w-[200px] min-w-[200px] max-w-[200px] overflow-hidden"
          showCreateTagButton
          hidedRepeatTags
          onChange={v => {
            const { value, label, ...tagInfo } = v as any;

            onTagSelectChange?.(v, tagItem?.tagInfo ?? tagInfo);
          }}
          disabled={isTagDisabled || loading}
          onChangeWithObject
          disableSelectList={disableSelectList}
          defaultShowName={
            tagItem?.annotation?.manual_feedback?.tag_key_name || ''
          }
        />
      </TooltipWhenDisabled>

      <div className="flex-1 overflow-hidden">
        <TagItem
          field={`${field}.annotation.manual_feedback`}
          tagItem={tagItem}
          onCreateAnnotationSuccess={onCreateAnnotationSuccess}
          span_id={span_id}
          trace_id={trace_id}
          onLoadingChange={v => {
            setLoading(v);
            setSaveLoading?.(v);
          }}
          isTagDisabled={isTagDisabled}
          start_time={start_time}
        />
      </div>
    </div>
  );
};
