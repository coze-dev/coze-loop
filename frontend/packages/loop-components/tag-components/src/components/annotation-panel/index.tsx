// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useEffect, useState } from 'react';

import { sendEvent, EVENT_NAMES } from '@cozeloop/tea-adapter';
import { I18n } from '@cozeloop/i18n-adapter';
import { useGuards, GuardPoint, GuardActionType } from '@cozeloop/guard';
import { ResizeSidesheet } from '@cozeloop/components';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { type span } from '@cozeloop/api-schema/observation';
import { IconCozCardPencil } from '@coze-arch/coze-design/icons';
import { Button } from '@coze-arch/coze-design';

import { AnnotationPanelContext } from './annotation-panel-context.ts';
import { AnnotationContent } from './annotation-content';

interface AnnotationPanelProps {
  span: span.OutputSpan;
  platformType?: string | number;
  onClose?: () => void;
}

export const AnnotationPanel = (props: AnnotationPanelProps) => {
  const { span, onClose } = props;
  const [visible, setVisible] = useState(false);
  const [saveLoading, setSaveLoading] = useState<boolean | undefined>(
    undefined,
  );
  const [editChanged, setEditChanged] = useState(false);
  const { spaceID } = useSpace();
  const guards = useGuards({
    points: [GuardPoint['ob.trace.annotation']],
  });

  const disableAnnotation =
    guards.data[GuardPoint['ob.trace.annotation']].type ===
    GuardActionType.READONLY;

  useEffect(() => {
    onClose?.();
  }, [span.span_id]);
  return (
    <AnnotationPanelContext.Provider
      value={{
        platformType: props.platformType,
        saveLoading,
        setSaveLoading,
        editChanged,
        setEditChanged,
      }}
    >
      <Button
        size="mini"
        className="!h-[32px] box-border text-[var(--coz-fg-plus)]"
        onClick={() => {
          setVisible(true);
          sendEvent(
            EVENT_NAMES.cozeloop_observation_manual_feedback_add_annotation,
            {
              space_id: spaceID,
            },
          );
        }}
        color="primary"
        disabled={disableAnnotation}
      >
        <IconCozCardPencil className="w-[14px] h-[14px]" />
        <span className="text-[14px] leading-[22px] font-medium ml-1">
          {I18n.t('annotation_data')}
        </span>
      </Button>
      <ResizeSidesheet
        dragOptions={{
          defaultWidth: 560,
          maxWidth: 800,
          minWidth: 440,
        }}
        visible={visible}
        onCancel={() => {
          setVisible(false);
          onClose?.();
        }}
        mask={false}
        disableScroll={false}
        closable={false}
        headerStyle={{
          padding: '0',
          paddingRight: '24px',
        }}
        title={
          <div className="h-[68px] px-6 box-border flex items-center border-0 border-solid border-b border-[var(--coz-stroke-primary)] gap-x-2">
            <span className="text-[18px] font-medium leading-[26px] text-[var(--coz-fg-plus)]">
              {I18n.t('annotation_data')}
            </span>

            <span
              className="text-[12px] leading-[16px] font-normal"
              style={{
                color: 'var(--coz-fg-dim)',
              }}
            >
              {editChanged
                ? saveLoading
                  ? I18n.t('tag_auto_saving')
                  : I18n.t('tag_saved_please_check_feedback')
                : null}
            </span>
          </div>
        }
      >
        <div className="pt-4 w-full h-full">
          <AnnotationContent span={span} />
        </div>
      </ResizeSidesheet>
    </AnnotationPanelContext.Provider>
  );
};
