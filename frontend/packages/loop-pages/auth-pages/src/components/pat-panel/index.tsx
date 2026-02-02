// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useState } from 'react';

import cls from 'classnames';
import { useRequest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { type PersonalAccessToken } from '@cozeloop/api-schema/foundation';
import { authnService } from '@cozeloop/account';
import { Button, Modal } from '@coze-arch/coze-design';

import { PatTable } from './pat-table';
import { PatModal, type PatInfo } from './pat-modal';
import { PatDetail } from './pat-detail';

import s from './index.module.less';

interface Props {
  className?: string;
}

const PAT_LINK_DOC =
  // cp-disable-next-line
  'https://www.coze.cn/open/docs/developer_guides/coze_api_overview';

const PAT_PAGE_SIZE = 100;

export function PatPanel({ className }: Props) {
  const [modalData, setModalData] = useState<{
    visible: boolean;
    value?: PatInfo;
  }>({ visible: false });

  const {
    data: patList,
    loading,
    refresh: reloadPatList,
  } = useRequest(() => authnService.listPat(1, PAT_PAGE_SIZE), {
    debounceWait: 200,
  });

  const openLinkDoc = () => {
    window.open(PAT_LINK_DOC);
  };

  const createPat = async (info: PatInfo) => {
    const { pat, token } = await authnService.createPat(
      info.duration === 'custom'
        ? {
            name: info.name,
            expire_at: info.expireAt
              ? Math.floor(info.expireAt.getTime() / 1000)
              : 0,
          }
        : { name: info.name, duration_day: info.duration },
    );

    reloadPatList();
    setModalData({ visible: false });
    Modal.info({
      title: I18n.t('new_pat'),
      footer: false,
      width: 560,
      closable: true,
      content: <PatDetail pat={pat} token={token} />,
    });
  };

  const editPat = (pat: PersonalAccessToken) => {
    setModalData({
      visible: true,
      value: {
        id: pat.id,
        name: pat.name,
        expireAt: new Date(Number(pat.expire_at) * 1000),
      },
    });
  };

  const updatePat = async (pat: PatInfo) => {
    if (!modalData.value?.id) {
      return;
    }

    await authnService.updatePat({ id: modalData.value?.id, name: pat.name });
    reloadPatList();
    setModalData({ visible: false });
  };

  const deletePat = async (id: string) => {
    await authnService.deletePat(id);
    reloadPatList();
  };

  return (
    <div className={cls(s.container, className)}>
      <div className={s.header}>
        <h3 className="flex-1 m-0">{I18n.t('auth_tab_pat')}</h3>
        <Button
          theme="solid"
          type="primary"
          onClick={() => setModalData({ visible: true })}
        >
          {I18n.t('add_token')}
        </Button>
      </div>
      <div className={s.tip}>
        <p>
          {I18n.t('pat_introduction')}
          <span className={s.link} onClick={openLinkDoc}>
            {I18n.t('api_instruction')}
          </span>
        </p>
        <p>{I18n.t('pat_reminder')}</p>
      </div>
      <PatTable
        dataSource={patList}
        loading={loading}
        onEdit={editPat}
        onDelete={deletePat}
      />
      {modalData.visible ? (
        <PatModal
          {...modalData}
          onCreate={createPat}
          onUpdate={updatePat}
          onCancel={() => setModalData({ visible: false })}
        />
      ) : null}
    </div>
  );
}
