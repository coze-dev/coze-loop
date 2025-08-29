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
      添加标签
    </Button>
  );
};
