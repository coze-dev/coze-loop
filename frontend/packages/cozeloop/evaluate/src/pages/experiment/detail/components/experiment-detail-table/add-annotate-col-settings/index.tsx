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
        人工标注管理
      </Button>
      <ResizeSidesheet
        title="人工标注管理"
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
