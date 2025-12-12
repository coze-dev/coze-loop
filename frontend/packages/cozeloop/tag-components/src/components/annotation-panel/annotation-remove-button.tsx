// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
import {
  type PlatformType,
  type DeleteManualAnnotationRequest,
} from '@cozeloop/api-schema/observation';
import { IconCozTrashCan } from '@coze-arch/coze-design/icons';
import { Button, Tooltip } from '@coze-arch/coze-design';

import { useDeleteAnnotation } from '@/hooks/use-delete-annotation';

import { useAnnotationPanelContext } from './annotation-panel-context.ts';

interface AnnotationRemoveButtonProps
  extends Omit<DeleteManualAnnotationRequest, 'workspace_id'> {
  onClick?: () => void;
  isRemoteItem?: boolean;
}

export const AnnotationRemoveButton = (props: AnnotationRemoveButtonProps) => {
  const { onClick, isRemoteItem, ...rest } = props;
  const { platformType } = useAnnotationPanelContext();

  const { runAsync, loading } = useDeleteAnnotation();

  return (
    <Tooltip content={I18n.t('delete')} theme="dark">
      <Button
        loading={loading}
        icon={<IconCozTrashCan />}
        onClick={() => {
          if (isRemoteItem) {
            runAsync({
              ...rest,
              platform_type: platformType as PlatformType,
            })
              .then(() => {
                onClick?.();
              })
              .catch(err => console.error(err));
            return;
          }
          onClick?.();
        }}
        color="secondary"
      />
    </Tooltip>
  );
};
