/* eslint-disable @typescript-eslint/naming-convention */
import createEle from 'crelt';
import { I18n } from '@cozeloop/i18n-adapter';

interface CountData {
  matchIndex: number;
  matchCount: number;
  searching?: boolean;
}

export class MatchCount {
  _data: CountData;
  dom: HTMLDivElement;
  constructor(
    data: CountData = { matchCount: 0, matchIndex: -1, searching: false },
  ) {
    this._data = data;
    this.dom = createEle('div', {
      class: 'cm-custom-search-match-count',
    }) as HTMLDivElement;

    this.update();
  }

  update() {
    if (this._data.matchCount > 0) {
      this.dom.innerText = ` ${this._data.matchIndex + 1} of ${
        this._data.matchCount > 1000 ? '1000+' : this._data.matchCount
      }`;

      this.dom.style.color = 'unset';
    } else {
      this.dom.innerText = I18n.t('prompt_no_results');

      if (this._data.searching) {
        this.dom.style.color = 'red';
      } else {
        this.dom.style.color = 'unset';
      }
    }
  }

  get data() {
    return this._data;
  }

  set data(data: CountData) {
    this._data = data;
    this.update();
  }
}
