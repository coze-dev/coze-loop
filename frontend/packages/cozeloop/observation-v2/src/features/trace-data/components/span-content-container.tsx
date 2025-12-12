/* eslint-disable react-hooks/exhaustive-deps */
import { useEffect, useState, useMemo } from 'react';

import { isObject } from 'lodash-es';
import cls from 'classnames';
import { useLatest } from 'ahooks';
import { type JsonViewerProps } from '@textea/json-viewer';
import { handleCopy as copy } from '@cozeloop/components';
import { type span } from '@cozeloop/api-schema/observation';
import { IconCozCopy, IconCozCorner } from '@coze-arch/coze-design/icons';
import { Button, SegmentTab, Tooltip } from '@coze-arch/coze-design';

import { beautifyJson, safeJsonParse } from '@/shared/utils/json';
import { useLocale } from '@/i18n';
import { useTraceDetailContext } from '@/features/trace-detail/hooks/use-trace-detail-context';
import { type TagType, type Span } from '@/features/trace-data/types';
import {
  FullscreenContext,
  useScrollView,
} from '@/features/trace-data/hooks/use-srcoll-view';

import styles from './index.module.less';

export enum TypeEnum {
  TEXT = 'TEXT',
  JSON = 'JSON',
}
interface SpanContentContainerProps {
  content: string | object;
  title: string;
  config?: Partial<JsonViewerProps>;
  hasBottomLine?: boolean;
  canSwitchRawType?: boolean;
  spanType?: string;
  tagType?: TagType;
  copyConfig: {
    moduleName?: string;
    point: string;
  };
  isEncryptionData?: boolean;
  spanID?: string;
  attrTos?: Span['attr_tos'];
  children: (
    renderType: TypeEnum,
    structuredContent: string | object,
  ) => React.ReactNode;
  hideSwitchRawType?: boolean;
  span: span.OutputSpan;
}
export const SpanContentContainer = (props: SpanContentContainerProps) => {
  const [showType, setShowType] = useState<TypeEnum>(TypeEnum.TEXT);
  const {
    content,
    title,
    hasBottomLine = true,
    spanID,
    children,
    hideSwitchRawType = false,
    span,
  } = props;
  const { t } = useLocale();
  const { customParams } = useTraceDetailContext();
  const { spanRenderConfig } = customParams ?? {};
  const latestSpanRenderConfig = useLatest(
    spanRenderConfig?.enableCopy ?? (() => true),
  );

  const { containerRef, isFullScreen, onFullScreenStateChange } =
    useScrollView();

  const enableCopy = latestSpanRenderConfig.current(span, props.tagType);

  const handleCopy = (data: object | string) => {
    let str = '';
    if (isObject(data)) {
      str = beautifyJson(data as object);
    } else {
      str = data as string;
    }
    copy(str);
  };
  const structuredContent = useMemo(
    () => (typeof content === 'string' ? safeJsonParse(content) : content),
    [content],
  );

  useEffect(() => {
    setShowType(TypeEnum.TEXT);
    onFullScreenStateChange(false);
  }, [spanID]);

  return (
    <FullscreenContext.Provider value={{ isFullScreen }}>
      <div
        ref={containerRef}
        style={{
          borderBottom: hasBottomLine ? '1px solid #1D1C2314' : 'none',
        }}
        className={cls('flex flex-col items-stretch px-[20px] py-3')}
      >
        <div className="flex items-center align-self-stretch justify-between h-8 mb-2">
          <div className="flex gap-1 items-center text-[16px] font-medium leading-[20px] text-[#000000]">
            <span className="mr-1">{title}</span>

            {structuredContent && enableCopy ? (
              <Tooltip content={t('copy_tooltip_text')} theme="dark">
                <Button
                  className="!w-[24px] !h-[24px] box-border mr-1"
                  size="small"
                  color="secondary"
                  icon={
                    <IconCozCopy className="flex items-center justify-center w-[14px] h-[14px] text-[var(--coz-fg-secondary)]" />
                  }
                  onClick={() => {
                    handleCopy(structuredContent);
                  }}
                />
              </Tooltip>
            ) : null}
            <Tooltip theme="dark" content="纯文本查看">
              <Button
                color="secondary"
                size="small"
                onClick={() => {
                  try {
                    const blob = new Blob(
                      [
                        typeof structuredContent === 'string'
                          ? structuredContent
                          : JSON.stringify(structuredContent),
                      ],
                      {
                        type: 'application/json',
                      },
                    );
                    const url = URL.createObjectURL(blob);
                    window.open(url, '_blank');
                  } catch (e) {
                    console.error('open raw content failed', e);
                  }
                }}
              >
                <span className="flex items-center justify-center w-[14px] h-[14px] text-[var(--coz-fg-secondary)]">
                  Raw
                </span>
              </Button>
            </Tooltip>
            {showType === TypeEnum.JSON ? (
              <Tooltip
                content={
                  isFullScreen
                    ? t('observation_collapse')
                    : t('observation_extend')
                }
                theme="dark"
              >
                <Button
                  size="small"
                  className="!w-[24px] !h-[24px] box-border"
                  color={isFullScreen ? 'primary' : 'secondary'}
                  icon={<IconCozCorner className={styles['copy-icon']} />}
                  onClick={() => {
                    onFullScreenStateChange(!isFullScreen);
                  }}
                />
              </Tooltip>
            ) : null}
          </div>
          <div>
            {!hideSwitchRawType && (
              <SegmentTab
                className={styles['segment-tab']}
                value={showType}
                size="small"
                onChange={event => {
                  setShowType(event.target.value as unknown as TypeEnum);
                }}
                options={[
                  { label: 'TEXT', value: TypeEnum.TEXT },
                  { label: 'JSON', value: TypeEnum.JSON },
                ]}
              />
            )}
          </div>
        </div>
        {children(showType, structuredContent)}
      </div>
    </FullscreenContext.Provider>
  );
};
