import { Button, Popover } from '@coze-arch/coze-design';

export const SkipButton = (props: {
  onClick: () => void;
  isShow: boolean;
  disabled: boolean;
}) => {
  const { onClick, isShow, disabled } = props;

  if (!isShow) {
    return null;
  }

  return (
    <Popover
      content="跳过评测对象执行配置，适用于评测集已包含agent实际输出的评测场景。"
      position="top"
      className="w-[320px] rounded-[8px] !py-2 !px-3"
    >
      <Button
        color="primary"
        onClick={onClick}
        // 有类型存在, 不允许点击跳过
        disabled={disabled}
      >
        跳过
      </Button>
    </Popover>
  );
};
