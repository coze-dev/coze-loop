import {
  type GetDatasetIOJobRequest,
  type ImportDatasetRequest,
} from '@cozeloop/api-schema/data';
import { DataApi } from '@cozeloop/api-schema';
export const useDataImportApi = () => {
  const importDataApi = (req: ImportDatasetRequest) =>
    DataApi.ImportDataset(req);
  const getDatasetIOJobApi = (req: GetDatasetIOJobRequest) =>
    DataApi.GetDatasetIOJob(req);
  return {
    importDataApi,
    getDatasetIOJobApi,
  };
};
