// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useMemo, useState, useEffect } from 'react';

import { nanoid } from 'nanoid';
import cls from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import { CodeEditor, IDRender } from '@cozeloop/components';
import { type EvaluatorOutputData } from '@cozeloop/api-schema/evaluation';
import {
  IconCozArrowDown,
  IconCozArrowRight,
  IconCozCheckMarkCircleFill,
  IconCozCrossCircleFill,
} from '@coze-arch/coze-design/icons';
import { Tag, Collapse, Spin, useFormState } from '@coze-arch/coze-design';

import HeaderItemsCount from './components/header-items-count';
import { TestDataSource, type TrialOperationResultsProps } from '../types';

import styles from './index.module.less';

const OpResultItem: React.FC<{
  result: EvaluatorOutputData & {
    key: string;
    item_id: string;
    dataMode: string;
  };
}> = ({ result }) => {
  const { evaluator_run_error, stdout, evaluator_result } = result;

  const isDatasetMode = result?.dataMode === TestDataSource.Dataset;

  const statusIcon = result.evaluator_run_error ? (
    <IconCozCrossCircleFill className="text-red-500" />
  ) : (
    <IconCozCheckMarkCircleFill className="text-green-500" />
  );
  const score = evaluator_result?.score;
  const statusColor = evaluator_run_error ? 'red' : 'green';

  const showText = useMemo(() => {
    let text = '';
    if (stdout) {
      text = stdout;
    }
    if (evaluator_run_error) {
      text = `${text}\n\n${evaluator_run_error?.message}`;
    }
    return text;
  }, [evaluator_run_error, stdout]);

  return (
    <Collapse.Panel
      itemKey={result.key}
      header={
        <div className="flex items-center w-full gap-4">
          <div className="flex items-center space-x-2 self-start mt-[2px]">
            <Tag
              color={statusColor}
              size="small"
              style={{ padding: 4, width: 20 }}
            >
              {statusIcon}
            </Tag>
            {result?.item_id && isDatasetMode ? (
              <IDRender id={result.item_id} useTag={true} enableCopy={false} />
            ) : null}
            {score !== undefined && (
              <span className="text-sm text-gray-600 w-8">
                {I18n.t('evaluate_score_points', { score })}
              </span>
            )}
          </div>
          <div className="text-gray-600 font-normal">
            {I18n.t('evaluate_reason_label')}
            {evaluator_result?.reasoning || I18n.t('system_error')}
          </div>
        </div>
      }
    >
      {stdout || evaluator_run_error?.message ? (
        <div
          style={{
            height: 226,
            borderTop: '1px solid var(--coz-stroke-primary)',
          }}
        >
          <CodeEditor
            language="json"
            value={showText}
            options={{
              readOnly: true,
              minimap: { enabled: false },
              scrollBeyondLastLine: false,
              wordWrap: 'on',
              fontSize: 12,
              lineNumbers: 'off',
              folding: false,
              automaticLayout: true,
            }}
            theme="vs-light"
            height="100%"
          />
        </div>
      ) : (
        <div
          className="font-medium text-gray-600 h-[52px] p-4 w-full text-center text-[var(--coz-fg-dim)]"
          style={{
            borderTop: '1px solid var(--coz-stroke-primary)',
          }}
        >
          {I18n.t('evaluate_no_run_output')}
        </div>
      )}
    </Collapse.Panel>
  );
};

export const OpResultsGroup: React.FC<{ results: EvaluatorOutputData[] }> = ({
  results,
}) => {
  const [activeKeys, setActiveKeys] = useState<string[]>([]);
  const [previousResultsLength, setPreviousResultsLength] = useState(0);
  const { values } = useFormState();

  const dataMode = values?.config?.testData?.source;

  const originSelectedData = values?.config?.testData?.originSelectedData || [];

  const memoizedResults = useMemo(
    () =>
      results.map((r, idx) => ({
        ...r,
        item_id: originSelectedData[idx]?.item_id,
        dataMode,
        key: nanoid(),
      })),
    [results, originSelectedData?.length, dataMode],
  );

  // 当 results 从空变为有值时，自动打开第一个 Panel
  useEffect(() => {
    if (
      results.length > 0 &&
      previousResultsLength === 0 &&
      memoizedResults.length > 0
    ) {
      setActiveKeys([memoizedResults[0].key]);
    }
    setPreviousResultsLength(results.length);
  }, [results.length, previousResultsLength, memoizedResults]);

  if (results.length === 0) {
    return (
      <div className="text-center py-8 text-gray-500">
        <p>{I18n.t('evaluate_no_run_result')}</p>
        <p className="text-sm mt-1">
          {I18n.t('evaluate_click_test_run_to_start')}
        </p>
      </div>
    );
  }

  return (
    <div className={styles.debugResultsGroup}>
      <Collapse
        expandIconPosition="left"
        expandIcon={<IconCozArrowRight />}
        collapseIcon={<IconCozArrowDown />}
        activeKey={activeKeys}
        onChange={keys => {
          if (Array.isArray(keys)) {
            setActiveKeys(keys);
          } else if (keys) {
            setActiveKeys([keys]);
          } else {
            setActiveKeys([]);
          }
        }}
      >
        {memoizedResults.map(result => (
          <OpResultItem key={result.key} result={result} />
        ))}
      </Collapse>
    </div>
  );
};

const RunningLoading = () => (
  <div className={styles.runningLoading}>
    <Spin spinning={true} size="small" />
    <span>{I18n.t('evaluate_test_running')}</span>
  </div>
);

export const TrialOperationResults: React.FC<
  TrialOperationResultsProps
> = props => {
  const { results = [], loading, className } = props;

  const successCount = useMemo(
    () => results?.filter(r => !r.evaluator_run_error).length,
    [results],
  );

  return (
    <div className={cls('flex flex-col h-full', className)}>
      {/* Header */}
      <div className="flex items-center border-b border-gray-200 h-[36px]">
        <h3 className="text-sm font-medium text-gray-900">
          {I18n.t('evaluate_test_run_result')}
        </h3>
        <HeaderItemsCount
          totalCount={results.length}
          successCount={successCount}
          failedCount={results.length - successCount}
        />
      </div>
      {/* Content */}
      {loading ? (
        <RunningLoading />
      ) : (
        <div className="flex-1 overflow-y-auto">
          <OpResultsGroup results={results} />
        </div>
      )}
    </div>
  );
};

export default TrialOperationResults;
// end_aigc
