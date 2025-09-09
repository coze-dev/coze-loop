import { I18n } from '@cozeloop/i18n-adapter';
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
      content={I18n.t('skip_eval_object_execution_config')}
      position="top"
      className="w-[320px] rounded-[8px] !py-2 !px-3"
    >
      <Button
        color="primary"
        onClick={onClick}
        // 有类型存在, 不允许点击跳过
        disabled={disabled}
      >
        {I18n.t('skip')}
      </Button>
    </Popover>
  );
};
