import { Guard, GuardPoint } from '@cozeloop/guard';
import { TableHeader } from '@cozeloop/components';
import { useNavigateModule } from '@cozeloop/biz-hooks-adapter';
import { type Experiment } from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import { Button, Modal } from '@coze-arch/coze-design';

import { verifyContrastExperiment } from '../../utils/experiment';

export function ExperimentRowSelectionActions({
  spaceID,
  experiments = [],
  onCancelSelect,
  onRefresh,
  onReportCompare,
}: {
  spaceID: Int64;
  experiments: Experiment[];
  onCancelSelect?: () => void;
  onRefresh?: () => void;
  onReportCompare?: (status: string) => void;
}) {
  const navigate = useNavigateModule();
  return (
    <TableHeader
      actions={
        <>
          <div className="text-xs">
            已选 {experiments.length} 条数据{' '}
            <span
              className="ml-1 text-[rgb(var(--coze-up-brand-9))] cursor-pointer"
              onClick={() => {
                onCancelSelect?.();
              }}
            >
              取消选择
            </span>
          </div>
          <Button
            color="primary"
            disabled={experiments.length < 2}
            onClick={() => {
              if (!verifyContrastExperiment(experiments)) {
                onReportCompare?.('fail');
                return;
              } else {
                onReportCompare?.('success');
                navigate(
                  `evaluation/experiments/contrast?experiment_ids=${experiments.map(experiment => experiment.id).join(',')}`,
                );
              }
            }}
          >
            实验对比
          </Button>

          <Guard point={GuardPoint['eval.experiments.batch_delete']}>
            <Button
              color="red"
              disabled={!experiments.length}
              onClick={() => {
                if (!experiments?.length) {
                  return;
                }
                Modal.confirm({
                  title: '批量删除实验',
                  content: `确认批量删除 ${experiments.length} 条实验数据吗？此修改将不可逆。`,
                  okText: '删除',
                  cancelText: '取消',
                  okButtonColor: 'red',
                  width: 420,
                  autoLoading: true,
                  async onOk() {
                    await StoneEvaluationApi.BatchDeleteExperiments({
                      workspace_id: spaceID,
                      expt_ids: experiments.map(item => item.id ?? ''),
                    });
                    onRefresh?.();
                  },
                });
              }}
            >
              删除
            </Button>
          </Guard>
        </>
      }
    />
  );
}
