import React from 'react';

import { handleCopy as copy } from '@cozeloop/components';
import { IconCozCopy } from '@coze-arch/coze-design/icons';
import { Tag, Collapse, Button, Tooltip } from '@coze-arch/coze-design';

import { safeJsonParse } from '@/shared/utils/json';
import { useLocale } from '@/i18n';

import { renderPlainText } from '../plain-text';
import { FieldWrapper } from './field-wrapper';

interface TraceItemProps {
  id: string;
  round: number;
  input: unknown;
  output: unknown;
  sectionRefs: React.MutableRefObject<Record<string, HTMLElement | null>>;
}

const TraceItem = ({
  id,
  round,
  input,
  output,
  sectionRefs,
}: TraceItemProps) => {
  const { t } = useLocale();
  const parsedInput = safeJsonParse(input as string);
  const parsedOutput = safeJsonParse(output as string);

  return (
    <div
      key={id}
      id={`anchor-${id}`}
      ref={el => (sectionRefs.current[`anchor-${id}`] = el)}
    >
      <Collapse.Panel
        className="w-full"
        itemKey={id}
        header={
          <div className="flex items-center w-full gap-x-1">
            <Tag color="grey" size="mini">
              {t('round', { round })}
            </Tag>
            {id ? (
              <>
                <span className="text-sm text-gray-500">
                  <span className="coz-fg-primary font-semibold">
                    Response ID
                  </span>
                  {' : '}
                  <span className="coz-fg-primary font-normal">{id}</span>
                </span>
                <Tooltip content={t('copy_id_tooltip')} theme="dark">
                  <Button
                    size="mini"
                    type="secondary"
                    color="secondary"
                    icon={<IconCozCopy />}
                    onClick={e => {
                      e.stopPropagation();
                      copy(id);
                    }}
                  />
                </Tooltip>
              </>
            ) : null}
          </div>
        }
      >
        <div className="mb-4">
          <FieldWrapper title="Input" onCopy={() => copy(input as string)}>
            {renderPlainText(parsedInput)}
          </FieldWrapper>
        </div>

        <div>
          <FieldWrapper title="Output" onCopy={() => copy(output as string)}>
            {renderPlainText(parsedOutput)}
          </FieldWrapper>
        </div>
      </Collapse.Panel>
    </div>
  );
};

export { TraceItem };
