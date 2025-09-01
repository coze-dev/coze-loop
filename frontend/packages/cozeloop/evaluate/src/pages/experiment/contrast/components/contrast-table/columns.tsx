import { TypographyText } from '@cozeloop/evaluate-components';
import { type Experiment } from '@cozeloop/api-schema/evaluation';
import { IconCozTrashCan } from '@coze-arch/coze-design/icons';
import { type ColumnProps, Popconfirm } from '@coze-arch/coze-design';

import { type ColumnInfo } from '@/types/experiment/experiment-contrast';
import IconButtonContainer from '@/components/common/icon-button-container';

import ExperimentResult from '../experiment-result';
import { type ExperimentContrastItem } from '../../utils/tools';

function ExperimentColumnHeader({
  experiment,
  index,
  enableDelete,
  onDelete,
}: {
  experiment: Experiment;
  index: number;
  enableDelete?: boolean;
  onDelete?: () => void;
}) {
  return (
    <div className="flex items-center">
      <TypographyText>
        {index === 0 ? '基准组' : `实验组 ${index}`} - {experiment.name}
      </TypographyText>
      {index !== 0 && enableDelete ? (
        <Popconfirm
          title="移除实验组"
          content={
            <>
              确认要移除 <span className="font-medium">{experiment.name}</span>{' '}
              吗？
            </>
          }
          okText="移除"
          cancelText="取消"
          showArrow={true}
          okButtonProps={{ color: 'red' }}
          onConfirm={onDelete}
        >
          <div className="ml-auto">
            <IconButtonContainer icon={<IconCozTrashCan />} />
          </div>
        </Popconfirm>
      ) : null}
    </div>
  );
}

/** 创建对比试验列配置 */
export function getExperimentContrastColumns(
  experiments: Experiment[] = [],
  {
    expand,
    spaceID,
    enableDelete,
    onExperimentChange,
    hiddenFieldMap,
    onRefresh,
    columnInfosMap,
  }: {
    spaceID?: Int64;
    onExperimentChange?: (experiments: Experiment[]) => void;
    expand?: boolean;
    enableDelete?: boolean;
    hiddenFieldMap?: Record<Int64, boolean>;
    onRefresh?: () => void;
    columnInfosMap?: Record<string, ColumnInfo[]>;
  } = {},
) {
  const columns = (experiments ?? []).map((experiment, index) => {
    const column: ColumnProps<ExperimentContrastItem> = {
      title: (
        <ExperimentColumnHeader
          experiment={experiment}
          index={index}
          enableDelete={enableDelete}
          onDelete={() =>
            onExperimentChange?.(experiments.filter(e => e !== experiment))
          }
        />
      ),
      dataIndex: `experimentResult.${experiment.id}`,
      // fixed: index === 0 ? true : undefined,
      align: 'left',
      width: 240,
      render: (_: unknown, record: ExperimentContrastItem) => {
        const result = record?.experimentResults?.[experiment?.id ?? ''];
        if (!result) {
          return '-';
        }

        return (
          <ExperimentResult
            expand={expand}
            result={result}
            experiment={experiment}
            hiddenFieldMap={hiddenFieldMap}
            spaceID={spaceID}
            onRefresh={onRefresh}
            columnInfos={columnInfosMap?.[experiment.id || '']}
          />
        );
      },
    };
    return column;
  });
  return columns;
}
