/* eslint-disable @coze-arch/use-error-in-catch */
import { type span } from '@cozeloop/api-schema/observation';
import { Button } from '@coze-arch/coze-design';

import { useLocale } from '@/i18n';
import { useTraceDetailContext } from '@/features/trace-detail/hooks/use-trace-detail-context';
import { useConfigContext } from '@/config-provider';

interface Props {
  span: span.OutputSpan;
}

export const JumpButton = ({ span }: Props) => {
  const { workspaceConfig, bizId } = useConfigContext();
  const {
    jumpButtonConfig = {
      visible: false,
    },
  } = useTraceDetailContext();
  const { t } = useLocale();

  if (bizId === 'fornax' || bizId === 'cozeloop') {
    return null;
  }
  const { workspaceId, domain: workspaceDomain } = workspaceConfig ?? {};

  const isLoop = workspaceDomain?.includes('loop');

  const tracePath = 'analytics/trace';

  let targetDomain: string | undefined = undefined;
  try {
    // 打包时候会注入相关变量
    targetDomain = workspaceDomain || process.env.DEFAULT_DOMAIN;
  } catch (e) {
    targetDomain = '';
  }

  const targetTraceLink = `${targetDomain}/space/${workspaceId}/${tracePath}`;

  if (!workspaceId || isLoop || !jumpButtonConfig.visible) {
    return null;
  }

  return (
    <Button
      size="mini"
      onClick={() => {
        if (jumpButtonConfig.onClick) {
          jumpButtonConfig.onClick?.(span);
        } else {
          window.open(targetTraceLink, '_blank');
        }
      }}
    >
      {jumpButtonConfig.text ?? t('jump_button_text')}
    </Button>
  );
};
