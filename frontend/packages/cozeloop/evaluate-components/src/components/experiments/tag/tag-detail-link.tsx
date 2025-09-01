import { useResourcePageJump } from '@cozeloop/biz-hooks-adapter';
import { IconCozLongArrowTopRight } from '@coze-arch/coze-design/icons';

export function TagDetailLink({ tagKey }: { tagKey?: string }) {
  const { getTagDetailURL } = useResourcePageJump();
  return (
    <span
      className="cursor-pointer text-brand-7"
      onClick={() => {
        window.open(getTagDetailURL(tagKey || ''));
      }}
    >
      查看标签详情
      <IconCozLongArrowTopRight className="ml-1" />
    </span>
  );
}
