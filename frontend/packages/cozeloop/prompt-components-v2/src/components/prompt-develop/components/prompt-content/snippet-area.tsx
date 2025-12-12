/* eslint-disable max-params */
import { useRef, useState } from 'react';

import { useShallow } from 'zustand/react/shallow';
import { Resizable } from 're-resizable';
import classNames from 'classnames';
import { useSize } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { TemplateType } from '@cozeloop/api-schema/prompt';
import { IconCozIllusEmpty } from '@coze-arch/coze-design/illustrations';
import { Typography } from '@coze-arch/coze-design';

import { usePromptStore } from '@/store/use-prompt-store';
import { useCompare } from '@/hooks/use-compare';
import { PromptDevLayout } from '@/components/prompt-dev-layout';

import { VariablesCard } from '../variables-card';
import { SegmentEditorCard } from '../prompt-editor-card/segment';

export function SnippetArea() {
  const editContainerRef = useRef(null);
  const [arrangeWidth, setArrangeWidth] = useState<Int64>('100%');
  const size = useSize(editContainerRef.current);
  const maxWidth = size?.width ? size.width - 400 : '50%';

  const { variables } = useCompare();
  const { templateType } = usePromptStore(
    useShallow(state => ({ templateType: state.templateType })),
  );

  return (
    <div className="flex flex-1 overflow-hidden w-full" ref={editContainerRef}>
      <Resizable
        size={{
          width: arrangeWidth,
          height: '100%',
        }}
        minWidth="525px"
        maxWidth={maxWidth}
        enable={{
          right: true,
        }}
        handleStyles={{
          right: {
            width: '4px',
            right: '-2px',
          },
        }}
        handleComponent={{
          right: (
            <div className="w-[2px] h-full border-0 border-solid border-brand-9 hover:border-l-2"></div>
          ),
        }}
        className={classNames('flex flex-col transition-all')}
        onResizeStop={(_e, _dir, _ref, d) => {
          setArrangeWidth(w => `calc(${w} + ${d.width}px)`);
        }}
      >
        <SegmentEditorCard />
      </Resizable>
      <PromptDevLayout
        className="box-border border-0 border-l border-solid flex flex-col gap-1 overflow-hidden bg-[#fcfcff] !flex-shrink-0 min-w-[400px] flex-1"
        title={I18n.t('variable')}
      >
        <div className="pl-6 pr-[18px] pb-4 overflow-auto h-full styled-scrollbar">
          {variables?.length || templateType?.type !== TemplateType.Normal ? (
            <VariablesCard onlyRenderContent contentClassName="!pt-0" />
          ) : (
            <div className="flex flex-col items-center justify-center gap-3 mt-[183px]">
              <IconCozIllusEmpty width="160" height="160" />
              <div className="flex flex-col items-center gap-1">
                <Typography.Text strong>
                  {I18n.t('no_variable')}
                </Typography.Text>
                <Typography.Text type="secondary" className="text-center">
                  {I18n.t('prompt_normal_template_var_intro')}
                </Typography.Text>
              </div>
            </div>
          )}
        </div>
      </PromptDevLayout>
    </div>
  );
}
