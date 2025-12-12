import { type Tool } from '@cozeloop/api-schema/prompt';

export type ToolWithMock = Tool & { mock_response?: string };

export interface ToolItemProps {
  data: ToolWithMock;
  showDelete?: boolean;
  onClick?: (data: ToolWithMock) => void;
  onDelete?: (name?: string) => void;
  disabled?: boolean;
}

export interface ToolModalProps {
  visible?: boolean;
  data?: ToolWithMock;
  disabled?: boolean;
  tools?: ToolWithMock[];
  onConfirm?: (
    data: ToolWithMock,
    isUpdate?: boolean,
    oldData?: ToolWithMock,
  ) => void;
  onClose?: () => void;
}
