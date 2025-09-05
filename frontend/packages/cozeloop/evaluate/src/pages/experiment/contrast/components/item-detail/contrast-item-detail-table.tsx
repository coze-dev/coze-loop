import { useEffect, useMemo, useState } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import {
  TypographyText,
  getExperimentNameWithIndex,
} from '@cozeloop/evaluate-components';
import { JumpIconButton } from '@cozeloop/components';
import { useBaseURL } from '@cozeloop/biz-hooks-adapter';
import {
  type FieldSchema,
  type Experiment,
} from '@cozeloop/api-schema/evaluation';
import {
  IconCozArrowRight,
  IconCozDubbleHorizontal,
  IconCozArrowDown,
} from '@coze-arch/coze-design/icons';
import { Dropdown, type ColumnProps } from '@coze-arch/coze-design';

import { getDatasetColumns } from '@/utils/experiment';
import { type ColumnInfo } from '@/types/experiment/experiment-contrast';
import { type DatasetRow } from '@/types';
import { ExperimentItemDetailTable } from '@/components/experiment';

import { type ExperimentContrastItem } from '../../utils/tools';
import ExperimentContrastResult from './experiment-contrast-result';

function ExperimentDetailColumnHeader({
  experiment,
  index,
}: {
  experiment: Experiment;
  index: number;
}) {
  const { baseURL } = useBaseURL();
  return (
    <div className="flex items-center gap-1">
      <TypographyText>
        {getExperimentNameWithIndex(experiment, index)}
      </TypographyText>
      <JumpIconButton
        className="ml-auto"
        onClick={() => {
          window.open(`${baseURL}/evaluation/experiments/${experiment?.id}`);
        }}
      />
    </div>
  );
}

/** 创建对比试验列配置 */
function getExperimentContrastDetailColumns({
  experiments,
  spaceID,
  onRefresh,
  columnInfosMap,
}: {
  experiments: Experiment[];
  spaceID: Int64;
  onRefresh?: () => void;
  columnInfosMap?: Record<string, ColumnInfo[]>;
}) {
  const columns = (experiments ?? []).map((experiment, index) => {
    const column: ColumnProps<ExperimentContrastItem> = {
      title: (
        <ExperimentDetailColumnHeader experiment={experiment} index={index} />
      ),
      dataIndex: `evaluatorsResult.${experiment.id}`,
      // fixed: index === 0 ? true : undefined,
      align: 'left',
      width: 440,
      render: (_: unknown, record: ExperimentContrastItem) => {
        const result = record?.experimentResults?.[experiment?.id ?? ''];
        if (!experiment?.id) {
          return '-';
        }
        return (
          <ExperimentContrastResult
            expand={true}
            result={result}
            experiment={experiment}
            columnInfos={columnInfosMap?.[experiment.id]}
            spaceID={spaceID}
            onRefresh={onRefresh}
          />
        );
      },
    };
    return column;
  });
  return columns;
}

export default function ContrastItemDetailTable({
  experiments = [],
  datasetFieldSchemas = [],
  datasetRow = {},
  experimentsDatasetRow = {},
  experimentContrastItem,
  spaceID,
  onRefresh,
  containerClassName,
  columnInfosMap,
}: {
  experiments: Experiment[];
  datasetFieldSchemas: FieldSchema[];
  datasetRow: DatasetRow;
  experimentsDatasetRow: Record<string, DatasetRow>;
  experimentContrastItem: ExperimentContrastItem;
  spaceID: Int64;
  onRefresh?: () => void;
  containerClassName?: string;
  columnInfosMap?: Record<string, ColumnInfo[]>;
}) {
  const [showDataset, setShowDataset] = useState(false);
  const [columns, setColumns] = useState<ColumnProps[]>([]);
  const [selectedExperiment, setSelectedExperiment] = useState<
    Experiment | undefined
  >();

  const datasetColumns = useMemo(
    () => getDatasetColumns(datasetFieldSchemas, { expand: true }),
    [datasetFieldSchemas],
  );

  const selectedExperimentText = getExperimentNameWithIndex(
    selectedExperiment || experiments[0],
    experiments.findIndex(item => item.id === selectedExperiment?.id),
    true,
  );

  useEffect(() => {
    setColumns(oldColumns => {
      if (experiments.length === 0 && oldColumns.length === 0) {
        return oldColumns;
      }
      const newColumns = getExperimentContrastDetailColumns({
        experiments,
        columnInfosMap,
        spaceID,
        onRefresh,
      });
      return newColumns;
    });
  }, [experiments, columnInfosMap, spaceID, experimentContrastItem]);

  return (
    <div
      className={`h-full flex flex-col overflow-hidden ${containerClassName ?? ''}`}
    >
      <div className="flex items-center shrink-0 bg-[var(--coz-mg-secondary)] py-3 px-5 text-sm font-medium">
        <TypographyText>{selectedExperimentText}</TypographyText>
        <span className="shrink-0 ml-1">
          {I18n.t('minus_evaluation_dataset')}
        </span>
        {showDataset ? (
          <Dropdown
            position="bottomLeft"
            menu={experiments.map((experiment, index) => ({
              node: 'item' as const,
              name: getExperimentNameWithIndex(experiment, index, true),
              onClick: () => {
                setSelectedExperiment(experiment);
              },
            }))}
          >
            <IconCozDubbleHorizontal className="cursor-pointer ml-2 text-[var(--coz-fg-secondary)] hover:text-[var(--coz-fg-primary)]" />
          </Dropdown>
        ) : null}
        <div
          className="ml-auto flex items-center gap-2 cursor-pointer"
          onClick={() => setShowDataset(!showDataset)}
        >
          <span className="text-xs font-normal">
            {showDataset ? I18n.t('collapse') : I18n.t('expand')}
          </span>
          {showDataset ? <IconCozArrowDown /> : <IconCozArrowRight />}
        </div>
      </div>
      {showDataset ? (
        <div className="overflow-auto shrink-0">
          <ExperimentItemDetailTable
            rowKey="turnID"
            columns={datasetColumns.filter(column => !column.hidden)}
            dataSource={
              selectedExperiment
                ? [experimentsDatasetRow[selectedExperiment?.id ?? '']]
                : [datasetRow]
            }
            weakHeader={true}
            tdClassName="text-[var(--coz-fg-secondary)]"
            thClassName="text-xs border-0 border-t border-[var(--coz-stroke-primary)] border-solid"
          />
        </div>
      ) : null}
      <div className="overflow-auto grow">
        <ExperimentItemDetailTable
          rowKey="turnID"
          columns={columns}
          dataSource={[experimentContrastItem]}
          tdClassName="!text-[var(--coz-fg-plus)]"
          thClassName="border-0 border-t border-b border-[var(--coz-stroke-primary)] border-solid"
          className="h-full"
        />
      </div>
    </div>
  );
}
