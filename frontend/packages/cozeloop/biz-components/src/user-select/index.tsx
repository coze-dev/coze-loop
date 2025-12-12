// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import cs from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import { UserProfile } from '@cozeloop/components';
import { useUserInfo } from '@cozeloop/biz-hooks-adapter';
import { Select, type SelectProps, Tag } from '@coze-arch/coze-design';

interface UserSelectProps {
  value?: string[];
  onChange?: (value?: string[]) => void;
  placeholder?: string;
  className?: string;
  valueProps?: 'user_id' | 'email_prefix';
}

export const UserSelect = ({
  value,
  onChange,
  placeholder,
  className = '',
  valueProps = 'user_id',
  ...rest
}: UserSelectProps & Omit<SelectProps, 'value' | 'onChange'>) => {
  const userInfo = useUserInfo();
  const DefaultOption = [
    {
      label: (
        <UserProfile
          className="ml-[6px]"
          avatarUrl={userInfo?.avatar_url}
          name={userInfo?.name}
        />
      ),

      value: userInfo?.user_id_str || '',
      data: {
        user_name: userInfo?.name,
        avatar_url: userInfo?.avatar_url,
      },
    },
  ];

  const renderSelectedItem = (optionNode, { onClose }) => {
    const content = (
      <Tag
        closable={true}
        onClose={onClose}
        color="primary"
        className="max-w-[130px] w-fit"
      >
        <UserProfile
          avatarUrl={optionNode?.data?.avatar_url}
          name={optionNode?.data?.user_name}
          avatarClassName="!h-[12px] !w-[12px]"
        />
      </Tag>
    );

    return {
      isRenderInTag: false,
      content,
    };
  };
  return (
    <Select
      className={cs('w-[200px]', className)}
      dropdownClassName={'w-[260px]'}
      placeholder={placeholder || I18n.t('search_creator')}
      multiple={true}
      defaultActiveFirstOption={false}
      autoClearSearchValue={false}
      maxTagCount={1}
      {...rest}
      renderSelectedItem={renderSelectedItem}
      value={value}
      onChange={res => {
        onChange?.(res as string[]);
      }}
    >
      <Select.OptGroup label={I18n.t('current_user')}>
        {DefaultOption.map(item => (
          <Select.Option key={item.value} value={item.value} data={item.data}>
            {item.label}
          </Select.Option>
        ))}
      </Select.OptGroup>
    </Select>
  );
};
