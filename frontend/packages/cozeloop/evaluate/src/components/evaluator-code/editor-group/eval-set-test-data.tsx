// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

import { useCallback, useState, useEffect } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import { type EvaluationSetItemTableData } from '@cozeloop/evaluate-components';
import { CodeEditor, IDRender } from '@cozeloop/components';
import {
  IconCozEmpty,
  IconCozPlus,
  IconCozTrashCan,
} from '@coze-arch/coze-design/icons';
import {
  Button,
  Empty,
  Collapse,
  IconButton,
  useFormState,
} from '@coze-arch/coze-design';

import type { TestDataItem } from '../types';
import TestDataModal from '../test-data-modal';

import styles from './index.module.less';

// 可折叠编辑器数组组件
const CollapsibleEditorArray: React.FC<{
  data: TestDataItem[];
  onChange: (
    data: TestDataItem[],
    originSelectedData?: EvaluationSetItemTableData[],
  ) => void;
  disabled?: boolean;
}> = ({ data, onChange, disabled }) => {
  const { values } = useFormState();

  const originSelectedData = values?.config?.testData?.originSelectedData || [];

  const handleDeleteItem = useCallback(
    (indexToDelete: number, event: React.MouseEvent) => {
      // 阻止事件冒泡，防止触发面板折叠/展开
      event.stopPropagation();

      // 创建新数组，移除指定索引的项
      const newData = data.filter((_, index) => index !== indexToDelete);

      const newOriginSelectedData = originSelectedData.filter(
        (_, index) => index !== indexToDelete,
      );

      onChange(newData, newOriginSelectedData);
    },
    [data, onChange],
  );

  const handleItemChange = useCallback(
    (index: number, value: string) => {
      try {
        const parsedValue = JSON.parse(value);
        const newData = [...data];
        newData[index] = parsedValue;
        onChange(newData);
      } catch (error) {
        // JSON 解析错误时不更新数据
        console.error('JSON 解析错误:', error);
      }
    },
    [data, onChange],
  );

  if (data.length === 0) {
    return null;
  }

  return (
    <div className="space-y-2">
      <Collapse expandIconPosition="left" className="flex flex-col gap-1">
        {data.map((item, index) => {
          const key = originSelectedData[index]?.item_id || `item-${index}`;
          return (
            <Collapse.Panel
              header={
                <div className="flex justify-between grow">
                  <IDRender id={key} useTag={true} enableCopy={false} />
                  <IconButton
                    size="mini"
                    icon={<IconCozTrashCan />}
                    disabled={disabled}
                    onClick={e => handleDeleteItem(index, e)}
                  />
                </div>
              }
              itemKey={key}
              className={styles.customCollapsePanelWrapper}
            >
              <div style={{ height: 284 }}>
                <CodeEditor
                  language="json"
                  value={JSON.stringify(item, null, 2)}
                  onChange={value => handleItemChange(index, value || '{}')}
                  options={{
                    minimap: { enabled: false },
                    scrollBeyondLastLine: false,
                    wordWrap: 'on',
                    fontSize: 12,
                    lineNumbers: 'on',
                    folding: true,
                    automaticLayout: true,
                    tabSize: 2,
                    insertSpaces: true,
                    readOnly: disabled,
                  }}
                  theme="vs-light"
                  height="100%"
                />
              </div>
            </Collapse.Panel>
          );
        })}
      </Collapse>
    </div>
  );
};

export interface EvalSetTestDataProps {
  testData?: TestDataItem[];
  onChange?: (data: TestDataItem[]) => void;
  disabled?: boolean;
  importedTestData: TestDataItem[];
  setImportedTestData: (
    data: TestDataItem[],
    originSelectedData?: EvaluationSetItemTableData[],
  ) => void;
  onOpenModalRef?: React.MutableRefObject<(() => void) | undefined>;
}

export const EvalSetTestData: React.FC<EvalSetTestDataProps> = ({
  onChange,
  disabled,
  importedTestData,
  setImportedTestData,
  onOpenModalRef,
}) => {
  const [modalVisible, setModalVisible] = useState(false);
  const { values } = useFormState();

  const originSelectedList = values?.config?.testData?.originSelectedData || [];

  // 打开测试数据构造弹窗
  const handleOpenModal = useCallback(() => {
    setModalVisible(true);
  }, []);

  // 将打开弹窗方法暴露给父组件
  useEffect(() => {
    if (onOpenModalRef) {
      onOpenModalRef.current = handleOpenModal;
    }
  }, [handleOpenModal, onOpenModalRef]);

  // 关闭测试数据构造弹窗
  const handleCloseModal = useCallback(() => {
    setModalVisible(false);
  }, []);

  // 导入测试数据
  const handleImportData = (
    data: TestDataItem[],
    originSelectedData?: EvaluationSetItemTableData[],
  ) => {
    setImportedTestData(
      [...importedTestData, ...data],
      [...originSelectedList, ...(originSelectedData || [])],
    );
    onChange?.(data);
    setModalVisible(false);
  };

  // 处理导入数据的修改
  const handleImportedDataChange = useCallback(
    (
      data: TestDataItem[],
      originSelectedData?: EvaluationSetItemTableData[],
    ) => {
      setImportedTestData(data, originSelectedData);
      onChange?.(data);
    },
    [onChange, setImportedTestData],
  );

  // 如果已经导入了数据，显示可折叠编辑器数组
  if (importedTestData.length > 0) {
    return (
      <>
        <div className="p-[10px] max-h-[600px] overflow-y-auto grow">
          <CollapsibleEditorArray
            data={importedTestData}
            onChange={handleImportedDataChange}
            disabled={disabled}
          />
        </div>

        {/* 测试数据构造弹窗 */}
        <TestDataModal
          visible={modalVisible}
          onClose={handleCloseModal}
          onImport={handleImportData}
          prevCount={importedTestData.length}
        />
      </>
    );
  }

  // 暂无测试数据状态
  return (
    <>
      <div className="flex flex-col items-center justify-center h-full p-8">
        <Empty
          image={
            <IconCozEmpty
              height="32px"
              width="32px"
              color="var(--coz-fg-dim, rgba(55, 67, 106, 0.38))"
            />
          }
          description={I18n.t('evaluate_no_test_data')}
        >
          <Button
            size="small"
            type="primary"
            className="mt-2"
            icon={<IconCozPlus />}
            onClick={handleOpenModal}
            disabled={disabled}
          >
            {I18n.t('construct_test_data')}
          </Button>
        </Empty>
      </div>

      {/* 测试数据构造弹窗 */}
      <TestDataModal
        visible={modalVisible}
        onClose={handleCloseModal}
        onImport={handleImportData}
        prevCount={importedTestData.length}
      />
    </>
  );
};

export default EvalSetTestData;
