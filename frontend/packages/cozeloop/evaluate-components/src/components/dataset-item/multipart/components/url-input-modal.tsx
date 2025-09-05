/* eslint-disable @coze-arch/max-line-per-function */
import React, { useState, useRef } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import { LoopTable } from '@cozeloop/components';
import {
  useImageUrlUpload,
  type UploadAttachmentDetail,
} from '@cozeloop/biz-hooks-adapter';
import { type Image as ImageProps } from '@cozeloop/api-schema/evaluation';
import {
  IconCozTrashCan,
  IconCozPlus,
  IconCozCheckMarkCircleFill,
  IconCozCrossCircleFill,
} from '@coze-arch/coze-design/icons';
import {
  Modal,
  Button,
  Image,
  Spin,
  Form,
  type FormApi,
  Typography,
  Tag,
  type ColumnProps,
} from '@coze-arch/coze-design';

import { ErrorTypeMap } from '@/const';

interface UrlInputModalProps {
  visible: boolean;
  onConfirm: (results: ImageProps[]) => void;
  onCancel: () => void;
  maxCount?: number;
}

export const UrlInputModal: React.FC<UrlInputModalProps> = ({
  visible,
  onConfirm,
  onCancel,
  maxCount = 6,
}) => {
  const [error, setError] = useState('');
  const [isUploading, setIsUploading] = useState(false);
  const [isUploaded, setIsUploaded] = useState(false);
  const [uploadResults, setUploadResults] = useState<UploadAttachmentDetail[]>(
    [],
  );
  const formRef = useRef<FormApi>();
  const { uploadImageUrl } = useImageUrlUpload();
  // Validate helpers
  const validateUrl = (url: string) => {
    try {
      new URL(url);
      return true;
    } catch {
      return false;
    }
  };

  // Upload logic
  const handleUpload = async formValues => {
    if (formValues?.urls?.length === 0) {
      setError(I18n.t('please_add_at_least_one_image_link'));
      return;
    }
    setIsUploading(true);
    setError('');
    try {
      const results = await uploadImageUrl(formValues?.urls);
      setUploadResults(results || []);
      setIsUploaded(true);
    } catch (err) {
      setError(I18n.t('upload_failed_please_try_again'));
    } finally {
      setIsUploading(false);
    }
  };

  const handleConfirm = async () => {
    if (isUploaded) {
      const successResults = uploadResults?.filter(
        item => item.errorType === undefined,
      );
      onConfirm(
        successResults.map(item => ({
          name: item.image?.name,
          url: item.originImage?.url,
          uri: item.image?.uri,
          thumb_url: item.image?.thumb_url,
        })),
      );
    } else {
      await formRef.current?.submitForm();
    }
  };

  const handleCancel = () => {
    setError('');
    setIsUploading(false);
    setIsUploaded(false);
    setUploadResults([]);
    onCancel();
  };

  // Dynamic form for URLs
  const renderInputStage = () => (
    <Form
      initValues={{ urls: [''] }}
      getFormApi={api => (formRef.current = api)}
      onSubmit={handleUpload}
    >
      {({ formState, formApi }) => {
        const urls: string[] = formState.values?.urls || [''];
        const canAdd = urls.length < maxCount;
        return (
          <div>
            {urls.map((url, idx) => (
              <div key={idx} className="flex  gap-2">
                <Form.Input
                  field={`urls[${idx}]`}
                  label={{
                    text: `${I18n.t('cozeloop_open_evaluate_image_placeholder1', { placeholder1: idx + 1 })}`,
                    required: true,
                  }}
                  fieldClassName="flex-1"
                  placeholder={I18n.t('please_enter_the_picture_link')}
                  rules={[
                    {
                      validator: (_, value, cb) => {
                        if (!value) {
                          cb(I18n.t('please_enter_the_picture_link'));
                          return false;
                        }
                        if (!validateUrl(value)) {
                          cb(I18n.t('please_enter_a_valid_url'));
                          return false;
                        }
                        return true;
                      },
                    },
                  ]}
                  className="w-full"
                />
                <Button
                  size="small"
                  color="secondary"
                  icon={<IconCozTrashCan className="w-[14px] h-[14px]" />}
                  onClick={() => {
                    formApi.setValue(
                      'urls',
                      urls.filter((_, i) => i !== idx),
                    );
                  }}
                  className="mt-[42px]"
                />
              </div>
            ))}
            <Button
              color="primary"
              disabled={!canAdd}
              icon={<IconCozPlus />}
              onClick={() => formApi.setValue('urls', [...urls, ''])}
              className="mt-2"
            >
              {I18n.t('space_member_role_type_add_btn')}{' '}
              <Typography.Text
                className="ml-1"
                type="secondary"
              >{`${urls.length}/${maxCount}`}</Typography.Text>
            </Button>
            {error ? (
              <div className="text-red-500 text-sm mt-1">{error}</div>
            ) : null}
          </div>
        );
      }}
    </Form>
  );

  const columns: ColumnProps<UploadAttachmentDetail>[] = [
    {
      title: I18n.t('image_address'),
      dataIndex: 'originImage.url',
      width: 220,
      ellipsis: true,
    },
    {
      title: I18n.t('image_preview'),
      dataIndex: 'originImage.url',
      width: 120,
      render: (url: string) => (
        <div className="flex items-center">
          <Image src={url} width={36} height={36} />
        </div>
      ),
    },
    {
      title: I18n.t('status'),
      key: 'status',
      align: 'left',
      width: 200,
      render: (record: UploadAttachmentDetail) => (
        <div className="flex items-center">
          <Tag
            prefixIcon={
              record?.errorType ? (
                <IconCozCrossCircleFill />
              ) : (
                <IconCozCheckMarkCircleFill />
              )
            }
            color={record?.errorType ? 'red' : 'green'}
          >
            {record?.errorType
              ? I18n.t('status_failed')
              : I18n.t('status_success')}
          </Tag>
          <Typography.Text type="secondary" className="ml-1">
            {record.errorType ? ErrorTypeMap[record.errorType] : ''}
          </Typography.Text>
        </div>
      ),
    },
  ];

  const renderResultStage = () => (
    <div className="space-y-4">
      <LoopTable
        tableProps={{
          columns,
          dataSource: uploadResults,
          rowKey: 'id',
          pagination: false,
          size: 'small',
        }}
      />
    </div>
  );

  const getConfirmButtonText = () => {
    if (isUploading) {
      return I18n.t('cozeloop_open_evaluate_uploading');
    }
    if (isUploaded) {
      return I18n.t('import_approved_images');
    }
    return I18n.t('upload');
  };
  const successCount = uploadResults?.filter(
    item => item.errorType === undefined,
  )?.length;
  return (
    <Modal
      title={I18n.t('add_image_link')}
      visible={visible}
      onCancel={handleCancel}
      width={640}
      footer={
        <div className="flex justify-end">
          <Button onClick={handleCancel} color="primary">
            {I18n.t('cancel')}
          </Button>
          <Button
            type="primary"
            onClick={handleConfirm}
            loading={isUploading}
            disabled={isUploaded && successCount === 0}
          >
            {getConfirmButtonText()}
          </Button>
        </div>
      }
    >
      {isUploading ? (
        <div className="flex items-center justify-center py-8">
          <Spin size="large" />
          <span className="ml-2">
            {I18n.t('cozeloop_open_evaluate_uploading_image')}
          </span>
        </div>
      ) : null}

      {!isUploading && !isUploaded && renderInputStage()}
      {!isUploading && isUploaded ? renderResultStage() : null}
    </Modal>
  );
};
