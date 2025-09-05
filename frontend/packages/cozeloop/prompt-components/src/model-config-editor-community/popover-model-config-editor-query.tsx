import { useModelList, useSpace } from '@cozeloop/biz-hooks-adapter';

import {
  type ModelConfigPopoverProps,
  PopoverModelConfigEditor,
} from './popover-model-config-editor';

export function PopoverModelConfigEditorQuery(
  props: Omit<ModelConfigPopoverProps, 'models'>,
) {
  const { spaceID } = useSpace();

  const service = useModelList(spaceID, props.scenario);

  return <PopoverModelConfigEditor {...props} models={service.data?.models} />;
}
