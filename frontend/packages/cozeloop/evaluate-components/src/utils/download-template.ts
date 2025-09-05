import Papa, { type UnparseObject } from 'papaparse';
import { I18n } from '@cozeloop/i18n-adapter';

export const downloadCSVTemplate = () => {
  try {
    const fields = ['input', 'reference_output'];
    const data = [
      [I18n.t('evaluate_biggest_animal_world'), I18n.t('evaluate_blue_whale')],
      [I18n.t('evaluate_living_habits_animal'), I18n.t('data_engine_eat_fish')],
    ];
    const templateJson: UnparseObject<string[]> = {
      fields,
      data,
    };
    const csv = Papa.unparse(templateJson);
    downloadCsv(csv, 'dataset template');
  } catch (error) {
    console.error(error);
  }
};
export function downloadCsv(csv: string, fileName: string) {
  try {
    const BOM = '\uFEFF';
    const file = new File([BOM, csv], fileName, {
      type: 'text/csv;charset=utf-8',
    });
    const anchor = document.createElement('a');
    anchor.download = fileName;
    anchor.href = URL.createObjectURL(file);
    anchor.click();
  } catch (err) {
    console.error(err);
  }
}
export const downloadWithUrl = async (src: string, filename: string) => {
  try {
    const response = await fetch(src);
    if (response.ok) {
      const blob = await response.blob();
      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = filename;
      link.click();
      URL.revokeObjectURL(url);
      link.remove();
    }
  } catch (error) {
    console.error(error);
  }
};
