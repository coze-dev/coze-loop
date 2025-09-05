import { I18n } from '@cozeloop/i18n-adapter';
import { Modal } from '@coze-arch/coze-design';

interface ImportTypeSectionProps {
  value: boolean;
  onChange?: (value: boolean) => void;
}
const importTypeList = [
  {
    label: I18n.t('evaluation_set_append_data'),
    value: false,
  },
  {
    label: I18n.t('evaluation_set_overwrite_data'),
    value: true,
  },
];

export const OverWriteField = ({ value, onChange }: ImportTypeSectionProps) => (
  <div className="flex gap-2">
    {importTypeList.map(type => (
      <div
        key={`${type.value}`}
        className={`flex-1 border cursor-pointer py-[4px] border-solid border-[rgb(var(--coze-up-brand-3))] rounded-[6px] flex items-center justify-center
           ${value === type.value ? '!border-[rgb(var(--coze-up-brand-9))] text-[rgb(var(--coze-up-brand-9))] bg-[rgb(var(--coze-up-brand-3))]' : ''}`}
        onClick={() => {
          if (type.value) {
            Modal.confirm({
              title: I18n.t('confirm_full_overwrite'),
              content: I18n.t('importing_data_will_overwrite_existing_data'),
              okText: I18n.t('global_btn_confirm'),
              cancelText: I18n.t('global_btn_cancel'),
              onOk: () => {
                onChange?.(type.value);
              },
              okButtonProps: {
                color: 'yellow',
              },
            });
          } else {
            onChange?.(type.value);
          }
        }}
      >
        {type.label}
      </div>
    ))}
  </div>
);
