import { useI18n } from '@cozeloop/components';
import { Modal, Switch, Typography } from '@coze-arch/coze-design';

import styles from './index.module.less';

interface GoogleSearchItemProps {
  onChange?: (val: boolean) => void;
  value?: boolean;
  disabled?: boolean;
}

export function GoogleSearchItem({
  onChange,
  value,
  disabled,
}: GoogleSearchItemProps) {
  const I18n = useI18n();
  const handleChange = (val: boolean) => {
    if (disabled) {
      return;
    }
    if (val) {
      Modal.confirm({
        title: I18n.t('task_delete_title'),
        content: I18n.t('fornax_prompt_disable_model_func_google_on'),
        onOk: () => {
          onChange?.(val);
        },
        okText: I18n.t('confirm'),
        cancelText: I18n.t('cancel'),
      });
    } else {
      onChange?.(val);
    }
  };

  return (
    <div className={styles['tools-list-item']}>
      <div className={styles['tools-list-item-header']}>
        <Typography.Text
          className="font-semibold variable-text"
          ellipsis={{ showTooltip: true }}
        >
          google_search
        </Typography.Text>
        <Switch
          size="small"
          checked={value}
          onChange={handleChange}
          disabled={disabled}
        />
      </div>
      <Typography.Text
        ellipsis={{
          showTooltip: {
            opts: {
              position: 'top',
              content: (
                <div className="max-h-[450px] overflow-y-auto overflow-x-hidden">
                  {I18n.t('fornax_prompt_model_built_in_methods')}
                </div>
              ),
            },
          },
        }}
        size="small"
        type="tertiary"
        className="w-full"
      >
        {I18n.t('fornax_prompt_model_built_in_methods')}
      </Typography.Text>
      <Typography.Text size="small" type="tertiary" className="w-full">
        {I18n.t('fornax_prompt_current_status')}
        {value ? I18n.t('fornax_prompt_enable') : I18n.t('close')}
      </Typography.Text>
    </div>
  );
}
