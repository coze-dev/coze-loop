import { Modal } from '@coze-arch/coze-design';

interface ImportTypeSectionProps {
  value: boolean;
  onChange?: (value: boolean) => void;
}
const importTypeList = [
  {
    label: '追加数据',
    value: false,
  },
  {
    label: '全量覆盖',
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
              title: '确认选择全量覆盖',
              content: '导入数据将覆盖现有数据',
              okText: '确认',
              cancelText: '取消',
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
