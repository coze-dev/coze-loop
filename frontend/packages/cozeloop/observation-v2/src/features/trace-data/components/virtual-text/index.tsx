import { Virtuoso } from 'react-virtuoso';
import { useRef } from 'react';

import cls from 'classnames';

import styles from './index.module.less';
export const VirtualText = ({
  text,
  className,
}: {
  text: string;
  className?: string;
}) => {
  const virtualRef = useRef(null);
  // 将text按照1000个字符切分
  const chunks = text?.match(/.{1,1000}/gs) || [];
  return (
    <Virtuoso
      ref={virtualRef}
      className={cls(styles.virtual, className)}
      data={chunks}
      itemContent={(index, item) => <span key={index}>{item}</span>}
    />
  );
};
