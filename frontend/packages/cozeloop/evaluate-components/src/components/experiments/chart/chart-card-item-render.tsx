import { useState } from 'react';

import classNames from 'classnames';
import { IconButtonContainer } from '@cozeloop/components';
import { IconCozExpand, IconCozInfoCircle } from '@coze-arch/coze-design/icons';
import { Card, Modal, Popover, Tooltip } from '@coze-arch/coze-design';

import styles from './index.module.less';

export interface ChartCardItem {
  id: string;
  title?: React.ReactNode | undefined;
  content?: React.ReactNode;
  fullContent?: React.ReactNode;
  tooltip?: React.ReactNode;
}

export default function ChartCardItemRender({
  item,
  action,
  cardBodyStyle = {},
  cardHeaderStyle = {},
  modalBodyStyle = {},
}: {
  item: ChartCardItem | undefined;
  action?: React.ReactNode;
  cardBodyStyle?: React.CSSProperties;
  cardHeaderStyle?: React.CSSProperties;
  modalBodyStyle?: React.CSSProperties;
}) {
  const [expand, setExpand] = useState(false);
  return (
    <>
      <Card
        className={classNames(
          'bg-[var(--coz-bg-max)]',
          styles['chart-card-item-render'],
        )}
        header={
          <div className="flex items-center gap-1 w-full font-medium">
            <div className="flex-1 min-w-0 font-medium">{item?.title}</div>
            {item?.tooltip ? (
              <Tooltip theme="dark" content={item?.tooltip}>
                <IconCozInfoCircle className="text-[var(--coz-fg-secondary)] hover:text-[var(--coz-fg-primary)] shrink-0" />
              </Tooltip>
            ) : null}
            <div className="shrink-0 flex items-center gap-1 flex-wrap ml-auto">
              {action}
              <IconButtonContainer
                icon={<IconCozExpand />}
                onClick={() => {
                  setExpand(true);
                }}
              />
            </div>
          </div>
        }
        headerStyle={{
          height: 56,
          display: 'flex',
          alignItems: 'center',
          ...cardHeaderStyle,
        }}
        bodyStyle={{
          padding: '4px 0',
          boxSizing: 'border-box',
          height: 276,
          ...cardBodyStyle,
        }}
      >
        <div className="w-full h-full">{item?.content}</div>
      </Card>
      {expand ? (
        <Modal
          visible={expand}
          onCancel={() => {
            setExpand(false);
          }}
          maskClosable={true}
          width={916}
          height={418}
          centered={true}
          motion={false}
          bodyStyle={modalBodyStyle}
          title={
            <div className="flex items-center gap-2">
              <div className="font-bold max-w-[800px]">{item?.title}</div>
              {item?.tooltip ? (
                <Popover content={<div className="p-2">{item?.tooltip}</div>}>
                  <IconCozInfoCircle className="text-xs font-normal text-[var(--coz-fg-secondary)] hover:text-[var(--coz-fg-primary)]" />
                </Popover>
              ) : null}
            </div>
          }
          size="large"
        >
          <div className="w-full h-full">
            {item?.fullContent || item?.content}
          </div>
        </Modal>
      ) : null}
    </>
  );
}
