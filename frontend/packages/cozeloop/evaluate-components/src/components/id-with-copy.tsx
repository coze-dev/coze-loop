import { type ReactNode } from 'react';

import { IconCozCopy } from '@coze-arch/coze-design/icons';
import { Button, Toast, Tooltip } from '@coze-arch/coze-design';

export default function IDWithCopy({
  id,
  showSuffixLength = 5,
  prefix,
}: {
  id: string;
  showSuffixLength?: number;
  prefix?: ReactNode;
}) {
  const idString = id?.toString() ?? '';
  const suffix = idString.slice(
    Math.max(idString.length - showSuffixLength, 0),
    idString.length,
  );
  return (
    <div className="flex items-center">
      <span className="shrink-0">#{suffix || '-'}</span>
      {prefix ? prefix : null}
      <span className="text-sm text-[var(--coz-fg-primary)] font-normal ml-2 mr-[2px]">
        数据项 ID
      </span>
      <Tooltip content={`复制 ${idString}`} theme="dark">
        <Button
          onClick={async e => {
            e.stopPropagation();
            try {
              await navigator.clipboard.writeText(idString);
              Toast.success({ content: '复制成功', top: 80 });
            } catch (error) {
              console.error(error);
              Toast.error({ content: '复制失败', top: 80 });
            }
          }}
          color="secondary"
          className="ml-[2px]"
          icon={<IconCozCopy className="text-sm" />}
          size="mini"
        />
      </Tooltip>
    </div>
  );
}
