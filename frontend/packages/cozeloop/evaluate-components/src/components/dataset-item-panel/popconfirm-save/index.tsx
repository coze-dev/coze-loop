import { Popconfirm, type PopconfirmProps } from '@coze-arch/coze-design';

interface Props extends PopconfirmProps {
  children?: React.ReactNode;
  needConfirm?: boolean;
}

export const PopconfirmSave: React.FC<Props> = props => {
  const { children, needConfirm, ...reset } = props;
  return props.needConfirm ? (
    <Popconfirm
      title="信息未保存"
      content="如不保存，已编辑的信息将会丢失。"
      okText="保存并继续"
      cancelText="不保存"
      {...reset}
    >
      {props.children}
    </Popconfirm>
  ) : (
    props.children
  );
};
