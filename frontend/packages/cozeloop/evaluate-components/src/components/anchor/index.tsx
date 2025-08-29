import cn from 'classnames';
import { type AnchorProps, Anchor } from '@coze-arch/coze-design';

import styles from './index.module.less';

export const LoopAnchor = (props: AnchorProps) => {
  const { className, ...rest } = props;
  return <Anchor className={cn(styles.anchor, className)} {...rest} />;
};
