// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
import { IconCozPlus } from '@coze-arch/coze-design/icons';
import { Button } from '@coze-arch/coze-design';

interface AnnotationAddButtonProps {
  disabled?: boolean;
  onAdd?: () => void;
}

export const AnnotationAddButton = (props: AnnotationAddButtonProps) => {
  const { disabled, onAdd } = props;

  return (
    <Button
      icon={<IconCozPlus className="w-[16px] h-[16px]" />}
      color="primary"
      disabled={disabled}
      onClick={() => {
        onAdd?.();
      }}
    >
      {I18n.t('add_tag')}
    </Button>
  );
};
