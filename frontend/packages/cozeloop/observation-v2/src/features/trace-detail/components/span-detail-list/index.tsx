import cls from 'classnames';
import { TextWithCopy } from '@cozeloop/components';
import { type span } from '@cozeloop/api-schema/observation';
import { Typography } from '@coze-arch/coze-design';

import { FieldList } from '@/shared/ui/field-list';
import { useLocale } from '@/i18n/useLocale';

import { getSpanTagList } from './config';

import styles from './index.module.less';

export interface Field {
  key: string;
  title: React.ReactNode;
  width?: number;
  item: React.ReactNode;
  enableCopy?: boolean;
}

interface FieldListProps extends React.HTMLAttributes<HTMLDivElement> {
  fields: Field[];
  span: span.OutputSpan;
  layout?: 'vertical' | 'horizontal';
  minColWidth?: number;
  maxColNum?: number;
}
export const SpanFieldList = ({
  fields,
  className,
  style,
  span,
  minColWidth,
  maxColNum,
  layout = 'vertical',
}: FieldListProps) => {
  const tagList = getSpanTagList(span);
  const { t } = useLocale();
  const totalFields = [...fields, ...tagList];

  const renderField = (field: Field) => {
    const { key, title, item, enableCopy } = field;
    return (
      <div
        className="justify-start flex-col flex-1 min-w-0 overflow-hidden inline-flex text-xs gap-1 w-full max-w-full"
        key={key}
      >
        <span className="text-[--coz-fg-dim] whitespace-nowrap flex items-center text-[13px]">
          {title}
        </span>
        <span className="overflow-hidden text-[13px] block max-w-full w-full text-[var(--coz-fg-primary)]">
          {typeof item === 'string' && enableCopy ? (
            <TextWithCopy
              content={item}
              textClassName="!font-[12px]"
              copyTooltipText={t('copy_tooltip_text')}
            />
          ) : (
            <Typography.Text
              className="!font-[12px]"
              ellipsis={{
                showTooltip: {
                  opts: {
                    theme: 'dark',
                  },
                },
              }}
            >
              {item}
            </Typography.Text>
          )}
        </span>
      </div>
    );
  };

  return layout === 'vertical' ? (
    <div
      className={cls(
        styles['field-list'],
        'flex flex-col gap-y-4 text-[13px]',
        className,
      )}
      style={style}
    >
      {totalFields.map(field => renderField(field))}
    </div>
  ) : (
    <div className="py-3 border-0 border-b border-solid border-[#1D1C2314]">
      <div className="text-sm text-black font-semibold leading-5 mb-2">
        {t('analytics_title_span_detail')}
      </div>
      <FieldList
        fields={totalFields}
        maxColNum={maxColNum ?? 2}
        minColWidth={minColWidth ?? 180}
      ></FieldList>
    </div>
  );
};
