import { IconCozRefresh } from '@coze-arch/coze-design/icons';
import { Button, Tooltip } from '@coze-arch/coze-design';

export function RefreshButton({
  onRefresh,
}: {
  onRefresh: (() => void) | undefined;
}) {
  return (
    <Tooltip content="刷新" theme="dark">
      <Button
        color="primary"
        icon={<IconCozRefresh />}
        onClick={() => onRefresh?.()}
      />
    </Tooltip>
  );
}
