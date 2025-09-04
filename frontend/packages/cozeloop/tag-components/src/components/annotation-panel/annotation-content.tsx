/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable complexity */
/* eslint-disable @typescript-eslint/no-explicit-any */
import { useEffect, useMemo, useRef, useState } from 'react';

import { isEmpty } from 'lodash-es';
import { I18n } from '@cozeloop/i18n-adapter';
import { type PlatformType, type span } from '@cozeloop/api-schema/observation';
import { annotation } from '@cozeloop/api-schema/observation';
import { tag } from '@cozeloop/api-schema/data';
import { IconCozInfoCircle } from '@coze-arch/coze-design/icons';
import { Form, ArrayField, Tooltip, Spin } from '@coze-arch/coze-design';

import { useDeleteAnnotation } from '@/hooks/use-delete-annotation';
import { useBatchGetTags } from '@/hooks/use-batch-get-tags';
import { MAX_TAG_LENGTH } from '@/const';

import { AnnotationRemoveButton } from './annotation-remove-button';
import { useAnnotationPanelContext } from './annotation-panel-context.ts';
import { AnnotationLabel } from './annotation-label';
import { AnnotationAddButton } from './annotation-add-button';

const { AnnotationType } = annotation;

export interface AnnotationContentProps {
  span: span.OutputSpan;
}

export interface CreateAnnotationFormValues {
  tags: {
    tagInfo: tag.TagInfo | undefined;
    annotation: annotation.Annotation | undefined;
    isRemoteValue?: boolean;
  }[];
}

export const AnnotationContent = (props: AnnotationContentProps) => {
  const { span } = props;
  const manualFeedback = useMemo(
    () =>
      span.annotations?.filter(an => an.type === AnnotationType.ManualFeedback),
    [span],
  );

  const [initValues, setInitValues] = useState<CreateAnnotationFormValues>({
    tags: [],
  });

  const { loading, runAsync } = useBatchGetTags();
  const [dataReady, setDataReady] = useState(false);
  const { runAsync: deleteAnnotation } = useDeleteAnnotation();
  const { platformType } = useAnnotationPanelContext();

  useEffect(() => {
    setDataReady(false);
    const tagKeyIds =
      manualFeedback?.map(an => an.manual_feedback?.tag_key_id ?? '') ?? [];
    if (isEmpty(tagKeyIds)) {
      setInitValues({ tags: [] });
      setTimeout(() => {
        setDataReady(true);
      }, 0);

      return;
    }
    runAsync(tagKeyIds).then(d => {
      const initVal = {
        tags:
          manualFeedback?.map(an => {
            const { tag_key_id } = an.manual_feedback ?? {};
            const tagInfo = d?.tag_info_list?.find(
              t => t.tag_key_id === tag_key_id,
            );
            return {
              tagInfo,
              annotation: {
                ...an,
                manual_feedback: {
                  ...(an.manual_feedback ?? {}),
                  template_tag_key_id: tagInfo?.tag_key_id ?? '',
                  tag_key_id: an.manual_feedback?.tag_key_id ?? '',
                  tag_key_name: an.manual_feedback?.tag_key_name ?? '',
                },
              },
              isRemoteValue: true,
            };
          }) ?? [],
      };
      setInitValues(initVal);
      setDataReady(true);
    });
  }, [manualFeedback, span.span_id]);

  const formRef = useRef<Form>(null);

  if (loading || !dataReady) {
    return (
      <div className="w-full h-full flex items-center justify-center">
        <Spin />
      </div>
    );
  }

  return (
    <Form<CreateAnnotationFormValues>
      initValues={initValues}
      ref={formRef}
      className="pb-[100px]"
    >
      {({ formState, formApi }) => (
        <>
          <div className="text-xxl font-semibold leading-6 text-[var(--coz-fg-plus)] mb-4 flex items-center gap-x-[6px]">
            {I18n.t('tag_list')}
            <span className="text-[12px] font-semibold leading-4 text-[var(--coz-fg-dim)]">
              {formState.values.tags?.length ?? 0} / 50
            </span>
          </div>
          <ArrayField field="tags">
            {({ arrayFields, add }) => (
              <div>
                {arrayFields.map(({ field, remove, key }, index) => {
                  const tagItem = formState.values.tags?.[index];
                  const disableSelectList = formState.values.tags?.map(
                    item => item.tagInfo?.tag_key_id ?? '',
                  );
                  return (
                    <div key={key} className="flex flex-col mb-[20px]">
                      <div className="flex items-center gap-x-2 text-[13px] font-normal leading-5 text-[var(--coz-fg-secondary)]">
                        <span>
                          {I18n.t('tag_number_with_placeholder', {
                            number: index + 1,
                          })}
                        </span>
                        <Tooltip
                          theme="dark"
                          content={
                            tagItem?.tagInfo?.description ??
                            I18n.t('no_tag_description')
                          }
                        >
                          <IconCozInfoCircle className="w-[14px] h-[14px]" />
                        </Tooltip>
                      </div>
                      <div className="flex items-center gap-x-2 w-full overflow-hidden">
                        <AnnotationLabel
                          span_id={span.span_id}
                          trace_id={span.trace_id}
                          start_time={span.started_at}
                          field={field}
                          tagItem={tagItem}
                          onCreateAnnotationSuccess={(_value, annotationId) => {
                            if (tagItem) {
                              tagItem.isRemoteValue = true;
                              if (annotationId) {
                                tagItem.annotation.id = annotationId;
                              }
                            }
                          }}
                          isTagDisabled={
                            tagItem?.tagInfo?.status === tag.TagStatus.Inactive
                          }
                          disableSelectList={disableSelectList}
                          onTagSelectChange={(v, extraInfo) => {
                            const { value, label, ...tagInfo } = v as any;

                            const isCreateNewTag = !formState.values.tags?.some(
                              item =>
                                item.tagInfo?.tag_key_id ===
                                extraInfo?.tag_key_id,
                            );

                            if (isCreateNewTag) {
                              formApi.setValue(
                                `${field}.annotation.manual_feedback.tag_key_id`,
                                value,
                              );
                              formApi.setValue(`${field}.tagInfo`, extraInfo);
                            } else {
                              if (
                                tagItem?.annotation?.id &&
                                (tagItem?.annotation?.manual_feedback
                                  ?.tag_key_id ||
                                  tagItem?.annotation?.key)
                              ) {
                                deleteAnnotation({
                                  annotation_id: tagItem?.annotation.id,
                                  span_id: span.span_id,
                                  start_time: span.started_at,
                                  trace_id: span.trace_id,
                                  annotation_key:
                                    tagItem?.annotation?.key ??
                                    tagItem?.annotation?.manual_feedback
                                      ?.tag_key_id,
                                  platform_type: platformType as PlatformType,
                                });
                              }

                              formApi.setValue(`${field}.tagInfo`, tagInfo);
                              formApi.setValue(
                                `${field}.annotation`,
                                undefined,
                              );
                              formApi.setValue(`${field}.isRemoteValue`, false);
                              formApi.setValue(
                                `${field}.annotation.manual_feedback.tag_key_id`,
                                value,
                              );
                              formApi.setValue(
                                `${field}.annotation.manual_feedback.tag_value`,
                                undefined,
                              );
                              formApi.setValue(
                                `${field}.annotation.manual_feedback.tag_value_id`,
                                undefined,
                              );
                            }
                          }}
                        />
                        <AnnotationRemoveButton
                          annotation_id={tagItem?.annotation?.id}
                          span_id={span.span_id}
                          start_time={span.started_at}
                          trace_id={span.trace_id}
                          annotation_key={
                            tagItem?.annotation?.key ??
                            tagItem?.annotation?.manual_feedback?.tag_key_id
                          }
                          onClick={() => remove()}
                          isRemoteItem={tagItem?.isRemoteValue ?? false}
                        />
                      </div>
                    </div>
                  );
                })}
                <AnnotationAddButton
                  onAdd={add}
                  disabled={
                    (formState.values.tags?.length ?? 0) >= MAX_TAG_LENGTH
                  }
                />
              </div>
            )}
          </ArrayField>
        </>
      )}
    </Form>
  );
};
