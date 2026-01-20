// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
interface JSONSchemaHeaderProps {
  disableChangeDatasetType: boolean;
  showAdditional: boolean;
}

export const JSONSchemaHeader = ({
  disableChangeDatasetType,
  showAdditional,
}: JSONSchemaHeaderProps) => (
  <div className="flex gap-3">
    <label className="semi-form-field-label semi-form-field-label-left semi-form-field-label-required flex-1 px-[18px]">
      <div className="semi-form-field-label-text" x-semi-prop="label">
        {I18n.t('name')}
      </div>
    </label>
    <label className="w-[160px] semi-form-field-label semi-form-field-label-left semi-form-field-label-required">
      <div className="semi-form-field-label-text" x-semi-prop="label">
        {I18n.t('data_type')}
      </div>
    </label>
    <label className=" w-[60px] pr-0 semi-form-field-label semi-form-field-label-left semi-form-field-label-required">
      <div className="semi-form-field-label-text" x-semi-prop="label">
        {I18n.t('required')}
      </div>
    </label>
    {showAdditional ? (
      <label className="w-[120px] semi-form-field-label semi-form-field-label-left semi-form-field-label-required">
        <div className="semi-form-field-label-text flex" x-semi-prop="label">
          {I18n.t('redundant_fields_allowed')}
        </div>
      </label>
    ) : null}
    {!disableChangeDatasetType ? <div className="w-[46px]"></div> : null}
  </div>
);
