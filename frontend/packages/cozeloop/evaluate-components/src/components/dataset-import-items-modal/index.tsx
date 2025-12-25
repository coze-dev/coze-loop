// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable max-lines-per-function */
/* eslint-disable complexity */
/* eslint-disable @coze-arch/max-line-per-function */
import { useRef, useState } from 'react';

import cs from 'classnames';
import { useRequest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { GuardPoint, useGuard } from '@cozeloop/guard';
import { InfoTooltip } from '@cozeloop/components';
import {
  useSpace,
  useDataImportApi,
  useDatasetTemplateDownload,
  FILE_FORMAT_MAP,
} from '@cozeloop/biz-hooks-adapter';
import { uploadFile } from '@cozeloop/biz-components-adapter';
import { type EvaluationSet } from '@cozeloop/api-schema/evaluation';
import { StorageProvider, FileFormat } from '@cozeloop/api-schema/data';
import { IconCozFileCsv } from '@coze-arch/coze-design/illustrations';
import { IconCozDownload, IconCozUpload } from '@coze-arch/coze-design/icons';
import {
  Button,
  Dropdown,
  // Button,
  Form,
  type FormApi,
  Modal,
  Loading,
  Typography,
  type UploadProps,
  withField,
} from '@coze-arch/coze-design';

import { getFileType, getFileHeaders } from '../../utils/upload';
import { getDefaultColumnMap } from '../../utils/import-file';
import { downloadWithUrl } from '../../utils/download-template';
import { useDatasetImportProgress } from './use-import-progress';
import { OverWriteField } from './overwrite-field';
import { ColumnMapField } from './column-map-field';

import styles from './index.module.less';
const FormColumnMapField = withField(ColumnMapField);
const FormOverWriteField = withField(OverWriteField);
export const DatasetImportItemsModal = ({
  onCancel,
  onOk,
  datasetDetail,
}: {
  onCancel: () => void;
  onOk: () => void;
  datasetDetail?: EvaluationSet;
}) => {
  const formRef = useRef<FormApi>();
  const { spaceID } = useSpace();
  const { importDataApi } = useDataImportApi();
  const [csvHeaders, setCsvHeaders] = useState<string[]>([]);
  const { startProgressTask, node } = useDatasetImportProgress(onOk);
  const [visible, setVisible] = useState(true);
  const [loading, setLoading] = useState(false);
  const { getDatasetTemplate } = useDatasetTemplateDownload();
  const guard = useGuard({ point: GuardPoint['eval.dataset.import'] });
  const dragSubTextRef = useRef<HTMLDivElement>(null);
  const [downloadingTemplateLoading, setDownloadingTemplateLoading] =
    useState<boolean>(false);
  const handleUploadFile: UploadProps['customRequest'] = async ({
    fileInstance,
    file,
    onProgress,
    onSuccess,
    onError,
  }) => {
    try {
      await uploadFile({
        file: fileInstance,
        fileType: fileInstance.type?.includes('image') ? 'image' : 'object',
        onProgress,
        onSuccess,
        onError,
        spaceID,
      });
    } catch (error) {
      console.error('upload file failed', error);
    }
    const fileType = getFileType(fileInstance?.name);
    formRef?.current?.setValue('fileType', fileType);
    const { headers, error } = await getFileHeaders(fileInstance);
    if (error) {
      formRef?.current?.setError('file', error);
    }
    if (headers) {
      setCsvHeaders(headers);
      formRef?.current?.setValue(
        'fieldMappings',
        getDefaultColumnMap(datasetDetail, headers),
      );
    }
  };
  const { data: templateUrlList } = useRequest(
    async () => {
      const res = await getDatasetTemplate({
        spaceID,
        datasetID: datasetDetail?.id as string,
      });
      return res?.map(item => ({
        label: `${I18n.t('cozeloop_open_evaluate_template_placeholder0', { placeholder0: FILE_FORMAT_MAP[item?.format || FileFormat.CSV] })}`,
        value: item.url,
      }));
    },
    {
      refreshDeps: [],
    },
  );
  const onSubmit = async values => {
    setLoading(true);
    try {
      const res = await importDataApi({
        workspace_id: spaceID,
        dataset_id: datasetDetail?.id as string,
        file: {
          provider: StorageProvider.S3,
          path: values.file?.[0]?.response?.Uri,
          ...(values?.fileType === FileFormat.ZIP
            ? {
                compress_format: FileFormat.ZIP,
                format: FileFormat.CSV,
              }
            : {
                format: values.fileType || FileFormat.CSV,
              }),
        },
        field_mappings: values.fieldMappings?.filter(item => !!item?.source),
        option: {
          overwrite_dataset: values.overwrite,
        },
      });
      if (res.job_id) {
        startProgressTask(res.job_id);
        setVisible(false);
      }
    } catch (error) {
      console.error('import data failed', error);
    } finally {
      setLoading(false);
    }
  };
  return (
    <>
      <Modal
        title={I18n.t('import_data')}
        width={640}
        visible={visible}
        keepDOM={true}
        onCancel={onCancel}
        className={styles.modal}
        hasScroll={false}
        footer={null}
      >
        <Form
          initValues={{
            fieldMappings: getDefaultColumnMap(datasetDetail, csvHeaders),
            overwrite: false,
            fileType: '',
          }}
          getFormApi={formApi => {
            formRef.current = formApi;
          }}
          onValueChange={values => {
            console.log('values', values);
          }}
          onSubmit={onSubmit}
        >
          {({ formState, formApi }) => {
            const file = formState.values?.file;
            const fieldMappings = formState.values?.fieldMappings;
            const disableImport =
              !file?.[0]?.response?.Uri ||
              fieldMappings?.every(item => !item?.source);
            return (
              <>
                <div
                  className={cs(styles.form, 'styled-scrollbar relative')}
                  ref={dragSubTextRef}
                >
                  <Form.Upload
                    field="file"
                    label={I18n.t('upload_data')}
                    limit={1}
                    onChange={({ fileList }) => {
                      if (fileList.length === 0) {
                        setCsvHeaders([]);
                        formRef?.current?.setValue(
                          'fieldMappings',
                          getDefaultColumnMap(datasetDetail, []),
                        );
                      }
                    }}
                    draggable={true}
                    previewFile={() => (
                      <IconCozFileCsv className="w-[32px] h-[32px]" />
                    )}
                    className={styles.upload}
                    dragIcon={<IconCozUpload className="w-[32px] h-[32px]" />}
                    dragMainText={I18n.t('click_or_drag_file_to_upload')}
                    dragSubText={
                      <div className="relative flex items-center">
                        <Typography.Text
                          className="!coz-fg-secondary"
                          size="small"
                        >
                          {I18n.t('evaluation_set_import_files_tips')}
                        </Typography.Text>
                        {templateUrlList?.length ? (
                          <Dropdown
                            getPopupContainer={() =>
                              dragSubTextRef.current || document.body
                            }
                            zIndex={100000}
                            position="bottom"
                            render={
                              <div
                                onClick={e => {
                                  e.stopPropagation();
                                }}
                              >
                                <Dropdown.Menu>
                                  {templateUrlList?.map(item => (
                                    <Dropdown.Item
                                      className="!pl-2"
                                      key={item.value}
                                      onClick={async () => {
                                        setDownloadingTemplateLoading(true);
                                        await downloadWithUrl(
                                          item.value || '',
                                          item.label,
                                        );
                                        setDownloadingTemplateLoading(false);
                                      }}
                                    >
                                      {item.label}
                                    </Dropdown.Item>
                                  ))}
                                </Dropdown.Menu>
                              </div>
                            }
                          >
                            <div
                              onClick={e => {
                                e.stopPropagation();
                              }}
                            >
                              <Typography.Text
                                link
                                icon={<IconCozDownload />}
                                className="ml-[12px]"
                                size="small"
                              >
                                {I18n.t('download_template')}
                                {downloadingTemplateLoading ? (
                                  <Loading
                                    loading
                                    size="mini"
                                    color="blue"
                                    className="w-[14px] pl-1 !h-[4px] coz-fg-primary"
                                  />
                                ) : null}
                              </Typography.Text>
                            </div>
                          </Dropdown>
                        ) : null}
                      </div>
                    }
                    action=""
                    accept=".csv, .zip, .xlsx, .xls"
                    customRequest={handleUploadFile}
                    rules={[
                      {
                        required: true,
                        message: I18n.t('upload_file'),
                      },
                    ]}
                  ></Form.Upload>
                  {file?.[0]?.response?.Uri ? (
                    <Form.Slot
                      className="form-mini"
                      label={{
                        text: (
                          <div className="inline-flex items-center gap-1 !coz-fg-primary">
                            <div>{I18n.t('column_mapping')}</div>
                            <InfoTooltip
                              className="h-[15px]"
                              content={I18n.t('source_column_mapping')}
                            />
                          </div>
                        ),

                        required: true,
                      }}
                    >
                      <Typography.Text
                        type="secondary"
                        size="small"
                        className="!coz-fg-secondary block"
                      >
                        {I18n.t('no_mapping_no_import')}
                      </Typography.Text>
                      {formState?.values?.fieldMappings?.map((field, index) => (
                        <FormColumnMapField
                          field={`fieldMappings[${index}]`}
                          noLabel
                          sourceColumns={csvHeaders}
                          rules={[
                            {
                              validator: (_, data, cb) => {
                                if (
                                  !data?.source &&
                                  data?.fieldSchema?.isRequired
                                ) {
                                  cb(
                                    I18n.t(
                                      'please_configure_the_import_column',
                                    ),
                                  );
                                  return false;
                                }
                                return true;
                              },
                            },
                          ]}
                        />
                      ))}
                    </Form.Slot>
                  ) : null}
                  <FormOverWriteField
                    field="overwrite"
                    rules={[
                      {
                        required: true,
                        message: I18n.t('select_import_method'),
                      },
                    ]}
                    label={I18n.t('import_method')}
                  />

                  <div className="h-6" />
                </div>
                <div className="flex justify-end px-6">
                  <Button
                    className="mr-2"
                    color="primary"
                    onClick={() => {
                      onCancel();
                    }}
                  >
                    {I18n.t('cancel')}
                  </Button>
                  <Button
                    color="brand"
                    onClick={() => {
                      formRef.current?.submitForm();
                    }}
                    loading={loading}
                    disabled={guard.data.readonly || disableImport}
                  >
                    {I18n.t('import')}
                  </Button>
                </div>
              </>
            );
          }}
        </Form>
      </Modal>
      {node}
    </>
  );
};
