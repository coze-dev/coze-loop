import { Typography } from '@coze-arch/coze-design';

export const TableCellText = ({ text }: { text: string }) => (
  <Typography.Text
    ellipsis={{ showTooltip: { opts: { theme: 'dark' } } }}
    className="text-[13px] font-normal leading-[20px]"
  >
    {text}
  </Typography.Text>
);
