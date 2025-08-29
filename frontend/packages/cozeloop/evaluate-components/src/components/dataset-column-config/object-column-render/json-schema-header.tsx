interface JSONSchemaHeaderProps {
  disabelChangeDatasetType: boolean;
  showAdditional: boolean;
}

export const JSONSchemaHeader = ({
  disabelChangeDatasetType,
  showAdditional,
}: JSONSchemaHeaderProps) => (
  <div className="flex gap-3">
    <label className="semi-form-field-label semi-form-field-label-left semi-form-field-label-required flex-1 px-[18px]">
      <div className="semi-form-field-label-text" x-semi-prop="label">
        名称
      </div>
    </label>
    <label className="w-[160px] semi-form-field-label semi-form-field-label-left semi-form-field-label-required">
      <div className="semi-form-field-label-text" x-semi-prop="label">
        数据类型
      </div>
    </label>
    <label className=" w-[60px] pr-0 semi-form-field-label semi-form-field-label-left semi-form-field-label-required">
      <div className="semi-form-field-label-text" x-semi-prop="label">
        必填
      </div>
    </label>
    {showAdditional ? (
      <label className="w-[120px] semi-form-field-label semi-form-field-label-left semi-form-field-label-required">
        <div className="semi-form-field-label-text flex" x-semi-prop="label">
          允许冗余字段
        </div>
      </label>
    ) : null}
    {!disabelChangeDatasetType ? <div className="w-[46px]"></div> : null}
  </div>
);
