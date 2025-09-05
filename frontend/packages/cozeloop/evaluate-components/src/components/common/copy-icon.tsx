import { I18n } from '@cozeloop/i18n-adapter';
import { handleCopy, IconButtonContainer } from '@cozeloop/components';
import { IconCozCopy } from '@coze-arch/coze-design/icons';
import { Tooltip } from '@coze-arch/coze-design';

export function CopyIcon({
  text,
  className,
  onClick,
}: {
  /** 待复制的文本 */
  text: string | undefined;
  className?: string;
  onClick?: (e: React.MouseEvent) => void;
}) {
  return (
    <Tooltip content={I18n.t('copy')} theme="dark">
      <div className={className}>
        <IconButtonContainer
          icon={<IconCozCopy />}
          onClick={event => {
            onClick?.(event);
            handleCopy(text ?? '');
          }}
        />
      </div>
    </Tooltip>
  );
}
