// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
import { ResizeSidesheet } from '@cozeloop/components';
import { useModalData } from '@cozeloop/base-hooks';
import { type ColumnAnnotation } from '@cozeloop/api-schema/evaluation';
import { Button } from '@coze-arch/coze-design';

import { AnnotateColSettings } from './annotate-col-settings';

interface Props {
  spaceID: string;
  experimentID: string;
  data?: ColumnAnnotation[];
  onAnnotateAdd?: () => void;
  onAnnotateDelete?: () => void;
}
export function AddAnnotateColumn({
  spaceID,
  experimentID,
  data = [],
  onAnnotateAdd,
  onAnnotateDelete,
}: Props) {
  const tagModal = useModalData();

  return (
    <>
      <Button color="primary" onClick={() => tagModal.open()}>
        {I18n.t('manual_annotation_management')}
      </Button>
      <ResizeSidesheet
        title={I18n.t('manual_annotation_management')}
        visible={tagModal.visible}
        onCancel={tagModal.close}
        width={680}
      >
        <AnnotateColSettings
          spaceID={spaceID}
          experimentID={experimentID}
          data={data}
          onAnnotateAdd={onAnnotateAdd}
          onAnnotateDelete={onAnnotateDelete}
        />
      </ResizeSidesheet>
    </>
  );
}
