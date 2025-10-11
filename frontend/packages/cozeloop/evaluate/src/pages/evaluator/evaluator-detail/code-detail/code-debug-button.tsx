import { IconCozPlayFill } from '@coze-arch/coze-design/icons';
import { Button } from '@coze-arch/coze-design';
import { I18n } from '@cozeloop/i18n-adapter';

interface CodeDebugButtonProps {
  onClick?: () => void;
  loading?: boolean;
}

export function CodeDebugButton({ onClick, loading }: CodeDebugButtonProps) {
  return (
    <Button
      icon={<IconCozPlayFill />}
      color="highlight"
      onClick={onClick}
      loading={loading}
    >
      {I18n.t('run')}
    </Button>
  );
}
