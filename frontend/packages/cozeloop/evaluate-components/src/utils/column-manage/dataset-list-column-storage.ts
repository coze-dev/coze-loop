export const DATASET_LIST_COLUMN_STORAGE_KEY = 'dataset-list-column';
export const getDatasetListColumnSortStorageKey = (spaceID: string) =>
  `${DATASET_LIST_COLUMN_STORAGE_KEY}-${spaceID}`;
