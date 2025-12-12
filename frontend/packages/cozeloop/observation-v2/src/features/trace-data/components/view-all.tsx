import { useState } from 'react';

import { LargeTxtRender } from '@cozeloop/components';
import { IconCozIllusNone } from '@coze-arch/coze-design/illustrations';
import { IconCozCross, IconCozDownload } from '@coze-arch/coze-design/icons';
import {
  Button,
  Divider,
  Spin,
  Typography,
  Empty,
} from '@coze-arch/coze-design';

import { fileDownload } from '@/shared/utils/download';
import { useLocale } from '@/i18n';
import { TagType, type Span } from '@/features/trace-data/types';
import { useFetchResource } from '@/features/trace-data/hooks/use-fetch-resource';

interface ViewAllProps {
  onViewAllClick: (show: boolean) => void;
  tagType?: string;
  attrTos?: Span['attr_tos'];
}

export const ViewAllModal = ({
  onViewAllClick,
  tagType,
  attrTos,
}: ViewAllProps) => {
  const [loading, setLoading] = useState(false);
  const handleDownload = async () => {
    try {
      setLoading(true);
      const url =
        tagType === 'input'
          ? attrTos?.input_data_url
          : attrTos?.output_data_url;
      const fileName = tagType === 'input' ? 'input' : 'output';
      if (url) {
        await fileDownload(url, fileName);
      }
    } catch (error) {
      console.error(error);
    } finally {
      setLoading(false);
    }
  };
  return (
    <div className="bg-white w-[1105px] h-[800px] max-w-[80%] max-h-[80%]  m-auto absolute top-[50%] left-[50%] -translate-x-[50%] -translate-y-[50%] z-[1000] box-border rounded-[8px] border border-solid border-[var(--coz-stroke-primary)]  flex flex-col shadow">
      <div className="h-[56px] flex justify-between items-center border-0 border-b border-solid border-[var(--coz-stroke-primary] px-3">
        <div className="flex items-center gap-x-2 text-[16px] font-semibold leading-[22px] text-[var(--coz-fg-primary)]">
          {tagType === TagType.Input ? 'Input' : 'Output'}
        </div>
        <div className="flex items-center">
          <Button
            className="w-6 h-6"
            icon={<IconCozDownload />}
            loading={loading}
            color="secondary"
            onClick={handleDownload}
          />
          <Divider className="h-[12px] w-[1px] bg-[var(--coz-stroke-plus)] ml-3 mr-1" />
          <Button
            className="w-6 h-6"
            icon={<IconCozCross />}
            color="secondary"
            onClick={() => onViewAllClick(false)}
          />
        </div>
      </div>
      <div className="px-3 py-2 flex-1 overflow-hidden">
        <InputOrOutputTextRender attrTos={attrTos} tagType={tagType} />
      </div>
    </div>
  );
};

interface InputOrOutputTextProps {
  attrTos: Span['attr_tos'];
  tagType?: string;
}
const InputOrOutputTextRender = (props: InputOrOutputTextProps) => {
  const { attrTos, tagType } = props;
  const { t } = useLocale();

  const url =
    tagType === TagType.Input
      ? attrTos?.input_data_url
      : attrTos?.output_data_url;
  const { text, loading, error } = useFetchResource(url);

  if (loading) {
    return (
      <div className="w-full h-full flex items-center justify-center">
        <Spin />
      </div>
    );
  }

  if (error) {
    return (
      <Typography.Text>
        <Empty
          title={t('current_content_unavailable')}
          description={t('tos_url_not_exist')}
          image={<IconCozIllusNone className="w-[120px] h-[120px]" />}
        />
      </Typography.Text>
    );
  }

  return <LargeTxtRender text={text} />;
};
