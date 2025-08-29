import cs from 'classnames';
import { Tag, type TagProps } from '@coze-arch/coze-design';

export const LoopTag = ({ children, ...rest }: TagProps) => (
  <Tag
    {...rest}
    className={cs(
      '!rounded-[3px] !px-[8px] !font-normal !h-[20px]',
      rest.className,
    )}
  >
    {children}
  </Tag>
);
