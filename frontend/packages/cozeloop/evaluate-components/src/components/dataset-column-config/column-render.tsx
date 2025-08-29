/* eslint-disable @coze-arch/max-line-per-function */
import cs from 'classnames';
import { TooltipWhenDisabled } from '@cozeloop/components';
import {
  IconCozArrowDown,
  IconCozArrowRight,
  IconCozCopy,
  IconCozTrashCan,
} from '@coze-arch/coze-design/icons';
import {
  Button,
  Collapse,
  FormInput,
  Popconfirm,
  Tooltip,
  Typography,
  useFieldApi,
  withField,
} from '@coze-arch/coze-design';

import {
  DataType,
  MUTIPART_DATA_TYPE_LIST_WITH_ARRAY,
} from '../dataset-item/type';
import { columnNameRuleValidator } from '../../utils/source-name-rule';
import { useColumnAdvanceConfig } from './object-column-render/use-column-advance-config';
import { RequiredField } from './object-column-render/required-field';
import { ObjectStructRender } from './object-column-render/object-struct-render';
import { AdditionalPropertyField } from './object-column-render/additional-property-field';
import { MultipartRender } from './multipart-column-render';
import { DataTypeSelect } from './field-type';
const FormDataTypeSelect = withField(DataTypeSelect);
const FormRequiredField = withField(RequiredField);
interface ColumnRenderProps {
  fieldKey: string;
  index: number;
  onDelete: () => void;
  onCopy: () => void;
  size?: 'large' | 'small';
  activeKey: string[];
  setActiveKey: (key: string[]) => void;
  disabledDataTypeSelect?: boolean;
}
const FormAdditionalPropertyField = withField(AdditionalPropertyField);

export const ColumnRender = ({
  fieldKey,
  index,
  onDelete,
  onCopy,
  size = 'large',
  activeKey,
  setActiveKey,
  disabledDataTypeSelect = false,
}: ColumnRenderProps) => {
  const typeField = useFieldApi(`${fieldKey}.${index}.type`);
  const keyField = useFieldApi(`${fieldKey}.${index}.key`);
  const nameField = useFieldApi(`${fieldKey}.${index}.name`);
  const columnField = useFieldApi(`${fieldKey}.${index}`);
  const allColumnField = useFieldApi(fieldKey);
  const type = typeField.getValue() as DataType;
  const isExist = keyField.getValue() !== undefined;
  const disabelChangeDatasetType = disabledDataTypeSelect && isExist;
  const { AdvanceConfigNode, showAdditional, inputType, isForm, isJSON } =
    useColumnAdvanceConfig({
      fieldKey: `${fieldKey}.${index}`,
      disabelChangeDatasetType,
    });
  const isObject = [DataType.Object, DataType.ArrayObject].includes(type);
  const getHeader = () => (
    <div className="flex w-full justify-between">
      <div className="flex items-center gap-[4px]">
        <Typography.Text className="text-[14px] !font-semibold">
          {nameField.getValue() || `列 ${index + 1}`}
        </Typography.Text>
        {activeKey.includes(`${index}`) ? (
          <IconCozArrowDown
            onClick={() =>
              setActiveKey(activeKey.filter(key => key !== `${index}`))
            }
            className="cursor-pointer w-[16px] h-[16px]"
          />
        ) : (
          <IconCozArrowRight
            onClick={() => setActiveKey([...activeKey, `${index}`])}
            className="cursor-pointer w-[16px] h-[16px]"
          />
        )}
      </div>
      <div onClick={e => e.stopPropagation()} className="flex  items-center">
        {AdvanceConfigNode}
        <Tooltip content="复制" theme="dark" className="mr-[2px]">
          <Button
            color="secondary"
            size="mini"
            icon={<IconCozCopy className="w-[14px] h-[14px]" />}
            onClick={() => onCopy()}
          ></Button>
        </Tooltip>
        {isExist ? (
          <Popconfirm
            content={
              <Typography.Text className="break-all text-[12px] !coz-fg-secondary">
                确认删除{' '}
                <Typography.Text className="!font-medium">
                  {nameField.getValue()}
                </Typography.Text>{' '}
                列，此操作不可逆
              </Typography.Text>
            }
            title="删除列"
            okText="删除"
            zIndex={1062}
            okButtonProps={{
              color: 'red',
            }}
            cancelText="取消"
            style={{ width: 280 }}
            onConfirm={() => {
              onDelete();
            }}
          >
            <Button
              icon={<IconCozTrashCan className="w-[14px] h-[14px]" />}
              color="secondary"
              size="mini"
            ></Button>
          </Popconfirm>
        ) : (
          <Tooltip content="删除" theme="dark">
            <Button
              icon={<IconCozTrashCan className="w-[14px] h-[14px]" />}
              color="secondary"
              size="mini"
              onClick={() => onDelete()}
            ></Button>
          </Tooltip>
        )}
      </div>
    </div>
  );

  return (
    <Collapse.Panel
      className="group"
      itemKey={`${index}`}
      header={getHeader()}
      showArrow={false}
    >
      <div className="flex flex-col justify-stretch">
        <div className="flex gap-[12px]">
          <FormInput
            fieldClassName="flex-1"
            label="名称"
            placeholder="请输入列名称"
            maxLength={50}
            autoComplete=""
            field={`${fieldKey}.${index}.name`}
            rules={[
              {
                required: true,
                message: '请输入列名称',
              },
              {
                validator: columnNameRuleValidator,
              },
              {
                validator: (_, value) => {
                  if (!value) {
                    return true;
                  }
                  const allColumnData = allColumnField.getValue();
                  // 判断之前的列名称中是否有与自己相同的name
                  const hasSameName = allColumnData
                    ?.slice(0, index)
                    ?.some(
                      (data, dataIndex) =>
                        dataIndex !== index && data.name === value,
                    );

                  return !hasSameName;
                },
                message: '列名称已存在',
              },
            ]}
          ></FormInput>
          <TooltipWhenDisabled
            disabled={disabelChangeDatasetType}
            content="草稿版本存在存量数据项，不支持修改数据类型。"
            theme="dark"
            className="top-9"
          >
            <FormDataTypeSelect
              label="数据类型"
              labelWidth={90}
              treeData={MUTIPART_DATA_TYPE_LIST_WITH_ARRAY}
              fieldClassName={'w-[190px]'}
              disabled={disabelChangeDatasetType || isJSON}
              onChange={newType => {
                columnField.setValue({
                  ...columnField.getValue(),
                  children: [],
                  schema: '',
                  additionalProperties: false,
                });
              }}
              field={`${fieldKey}.${index}.type`}
              className="w-full"
              rules={[{ required: true, message: '请选择数据类型' }]}
            ></FormDataTypeSelect>
          </TooltipWhenDisabled>
          <FormRequiredField
            label={{
              text: '必填',
              required: true,
            }}
            fieldClassName={'w-[60px]'}
            className="w-full"
            disabled={disabledDataTypeSelect}
            field={`${fieldKey}.${index}.isRequired`}
          />
          <FormAdditionalPropertyField
            disabled={disabelChangeDatasetType}
            label={{
              text: '允许冗余字段',
              required: true,
            }}
            fieldClassName={cs(
              'w-[120px]',
              isObject && isForm && showAdditional ? '' : 'hidden',
            )}
            className="w-full"
            field={`${fieldKey}.${index}.additionalProperties`}
          />
        </div>
        <div className="flex-grow-1">
          <FormInput
            label="描述"
            placeholder="请输入列描述"
            maxLength={200}
            field={`${fieldKey}.${index}.description`}
            autoComplete="off"
          ></FormInput>
        </div>
        {isObject ? (
          <ObjectStructRender
            key={type}
            inputType={inputType}
            showAdditional={showAdditional}
            fieldKey={`${fieldKey}.${index}`}
            disabelChangeDatasetType={disabelChangeDatasetType}
          />
        ) : null}
        {type === DataType.MultiPart ? (
          <MultipartRender inputType={inputType} />
        ) : null}
      </div>
    </Collapse.Panel>
  );
};
