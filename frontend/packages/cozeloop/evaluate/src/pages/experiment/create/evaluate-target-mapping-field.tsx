import { useMemo, type FC } from 'react';

import { getTypeText } from '@cozeloop/evaluate-components';
import { type FieldSchema } from '@cozeloop/api-schema/evaluation';
import { IconCozEmpty } from '@coze-arch/coze-design/icons';
import {
  EmptyState,
  Loading,
  withField,
  type CommonFieldProps,
} from '@coze-arch/coze-design';

import {
  type OptionSchema,
  type OptionGroup,
} from '@/components/mapping-item-field/types';
import { MappingItemField } from '@/components/mapping-item-field';

import emptyStyles from './empty-state.module.less';

export interface EvaluateTargetMappingProps {
  loading?: boolean;
  keySchemas?: FieldSchema[];
  evaluationSetSchemas?: FieldSchema[];
  prefixField: string;
  value?: Record<string, OptionSchema>;
  onChange?: (v?: Record<string, OptionSchema>) => void;
}

export const EvaluateTargetMappingField: FC<
  CommonFieldProps & EvaluateTargetMappingProps
> = withField(function ({
  loading,
  keySchemas,
  evaluationSetSchemas,
  prefixField,
  // value,
  // onChange,
  // ...props
}: EvaluateTargetMappingProps) {
  const optionGroups = useMemo(
    () =>
      evaluationSetSchemas
        ? [
            {
              schemaSourceType: 'set',
              children: evaluationSetSchemas?.map(s => ({
                ...s,
                schemaSourceType: 'set',
              })),
            } satisfies OptionGroup,
          ]
        : [],
    [evaluationSetSchemas],
  );
  if (loading) {
    return (
      <div className="h-[84px] w-full flex items-center justify-center">
        <Loading
          className="!w-full"
          size="large"
          label={'正在加载字段映射'}
          loading={true}
        ></Loading>
      </div>
    );
  }

  if (!keySchemas) {
    return (
      <div className="h-[84px] w-full flex items-center justify-center">
        <EmptyState
          size="default"
          icon={<IconCozEmpty className="coz-fg-dim text-32px" />}
          title="暂无数据"
          className={emptyStyles['empty-state']}
          // description="请选择评测对象和版本号后再查看"
        />
      </div>
    );
  }
  return (
    <div>
      {keySchemas?.map(k => (
        <MappingItemField
          key={k.name}
          noLabel
          field={`${prefixField}.${k.name}`}
          fieldClassName="!pt-0"
          keyTitle="评测对象"
          keySchema={k}
          optionGroups={optionGroups}
          rules={[
            {
              validator: (_rule, v) => {
                if (!v) {
                  return new Error('请选择');
                }
                if (getTypeText(v) !== getTypeText(k)) {
                  return new Error('所选字段数据类型不一致，请重新选择');
                }
                return true;
              },
            },
          ]}
        />
      ))}
    </div>
  );
});
