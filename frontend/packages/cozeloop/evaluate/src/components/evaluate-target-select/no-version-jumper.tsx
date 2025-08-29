import { IconCozLongArrowTopRight } from '@coze-arch/coze-design/icons';
import { Tag } from '@coze-arch/coze-design';

interface NoVersionJumperProps {
  targetUrl: string;
  isShowTag?: boolean;
}

const NoVersionJumper = (props: NoVersionJumperProps) => {
  const { targetUrl, isShowTag = true } = props;
  return (
    <div className="w-full flex flex-row items-center justify-between gap-1 pr-2">
      <div className="flex flex-row items-center gap-3">
        <div className="coz-fg-dim">{'草稿版本'}</div>
        {isShowTag ? (
          <Tag color="yellow" className="!h-5 !px-2 !py-[2px] rounded-[3px]">
            {'修改未提交'}
          </Tag>
        ) : null}
      </div>
      <div
        onClick={() => {
          window.open(targetUrl);
        }}
        className="flex-shrink-0 h-8 flex flex-row items-center cursor-pointer"
      >
        <div className="text-sm font-medium text-brand-9">{'去提交'}</div>
        <IconCozLongArrowTopRight className="h-4 w-4 text-brand-9 ml-1" />
      </div>
    </div>
  );
};

export default NoVersionJumper;
