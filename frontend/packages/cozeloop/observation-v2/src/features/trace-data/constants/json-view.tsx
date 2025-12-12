/* eslint-disable @typescript-eslint/naming-convention */
import { defineDataType, type JsonViewerProps } from '@textea/json-viewer';

import { PreviousResponseIdType } from '../components/json-data-type';

const previousResponseIdType = defineDataType<string>({
  is(value, path) {
    if (typeof value === 'string' && path[0] === 'previous_response_id') {
      return true;
    }

    return false;
  },

  Component: props => <PreviousResponseIdType {...props} />,
});

export type BuildInValueTypes = 'previousResponseId';

const buildInValueTypesMap = {
  previousResponseId: previousResponseIdType,
};

interface params {
  enabledValuesTypes?: BuildInValueTypes[];
}
export const getJsonViewConfig: (
  params?: params,
) => Partial<JsonViewerProps> = ({ enabledValuesTypes = [] } = {}) => {
  const valueTypes = enabledValuesTypes.map(type => buildInValueTypesMap[type]);

  return {
    rootName: false,
    displayDataTypes: false,
    indentWidth: 2,
    enableClipboard: false,
    collapseStringsAfterLength: 300,
    defaultInspectDepth: 5,
    style: {
      wordBreak: 'break-all',
    },
    valueTypes,
  };
};
