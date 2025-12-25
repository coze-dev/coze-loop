// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable max-len */

import { useState } from 'react';

import {
  promptDisplayColumns,
  PromptList,
  PromptDeleteModal,
  PromptCreateModal,
} from '@cozeloop/prompt-components-v2';
import { I18n } from '@cozeloop/i18n-adapter';
import { useModalData, useRefresh } from '@cozeloop/hooks';
import { TableColActions } from '@cozeloop/components';
import {
  useNavigateModule,
  useOpenWindow,
  useSpace,
  useUserInfo,
} from '@cozeloop/biz-hooks-adapter';
import { UserSelect } from '@cozeloop/biz-components-adapter';
import { type Prompt } from '@cozeloop/api-schema/prompt';
import { withField, type ColumnProps } from '@coze-arch/coze-design';

const FormUserSelect = withField(UserSelect);

export default function PromptListPage() {
  const navigate = useNavigateModule();
  const { openBlank } = useOpenWindow();
  const { spaceID } = useSpace();
  const { user_id_str } = useUserInfo();
  const [refreshFlag, refresh] = useRefresh();
  const [isCopyPrompt, setIsCopyPrompt] = useState(false);
  const createModal = useModalData<Prompt>();
  const [isSnippet, setIsSnippet] = useState(false);

  const deleteModal = useModalData<Prompt>();

  const operateCol: ColumnProps<Prompt> = {
    title: I18n.t('operation'),
    key: 'action',
    dataIndex: 'action',
    width: 160,
    align: 'left',
    fixed: 'right',
    render: (_: unknown, row: Prompt) => (
      <TableColActions
        actions={[
          {
            label: I18n.t('detail'),
            onClick: () => navigate(`pe/prompts/${row.id}`),
          },
          {
            label: I18n.t('prompt_call_records'),
            onClick: () =>
              openBlank(
                `observation/traces?relation=and&selected_span_type=root_span&trace_filters=%257B%2522query_and_or%2522%253A%2522and%2522%252C%2522filter_fields%2522%253A%255B%257B%2522field_name%2522%253A%2522prompt_key%2522%252C%2522logic_field_name_type%2522%253A%2522prompt_key%2522%252C%2522query_type%2522%253A%2522in%2522%252C%2522values%2522%253A%255B%2522${row.prompt_key}%2522%255D%257D%255D%257D&trace_platform=prompt`,
              ),
          },
          {
            label: I18n.t('edit'),
            onClick: () => createModal.open(row),
          },
          {
            label: I18n.t('copy'),
            onClick: () => {
              setIsCopyPrompt(true);
              createModal.open(row);
            },
            disabled: !row.prompt_basic?.latest_version,
            disabledTooltip: I18n.t('prompt_copy_draft_not_supported'),
          },
          {
            label: I18n.t('delete'),
            onClick: () => {
              if (row?.id) {
                deleteModal.open(row);
              }
            },
            type: 'danger',
            disabled: row.prompt_basic?.created_by !== user_id_str,
            disabledTooltip: I18n.t('prompt_no_delete_permission'),
          },
        ]}
      />
    ),
  };

  const newColumns = [...promptDisplayColumns, operateCol];

  const isEdit = Boolean(createModal.data?.id) && !isCopyPrompt;

  const onItemClick = (info?: Prompt, event?: React.MouseEvent) => {
    if (info?.id) {
      if (event?.shiftKey || event?.metaKey || event?.ctrlKey) {
        // Shift + 点击
        openBlank(`pe/prompts/${info.id}`);
      } else {
        // 点击
        navigate(`pe/prompts/${info.id}`);
      }
    }
  };

  return (
    <>
      <PromptList
        columns={newColumns}
        spaceID={spaceID}
        refreshFlag={refreshFlag}
        extraSearchFormItems={
          <FormUserSelect
            field="created_bys"
            placeholder={I18n.t('prompt_all_creators')}
            noLabel
          />
        }
        onCreatePromptClick={() => createModal.open()}
        onCreateSegmentClick={() => {
          createModal.open();
          setIsSnippet(true);
        }}
        onTableRow={(record: Prompt) => ({
          onClick: e => onItemClick(record, e),
        })}
      />

      <PromptCreateModal
        spaceID={spaceID}
        visible={createModal.visible}
        onCancel={() => {
          setIsCopyPrompt(false);
          createModal.close();
          setIsSnippet(false);
        }}
        data={createModal.data}
        isEdit={isEdit}
        isCopy={isCopyPrompt}
        isSnippet={isSnippet}
        onOk={res => {
          if (!isEdit) {
            onItemClick(res);
          }
          createModal.close();
          refresh();
          setIsCopyPrompt(false);
        }}
      />

      <PromptDeleteModal
        data={deleteModal.data}
        visible={deleteModal.visible}
        onCacnel={deleteModal.close}
        onOk={() => {
          deleteModal.close();
          refresh();
        }}
      />
    </>
  );
}
