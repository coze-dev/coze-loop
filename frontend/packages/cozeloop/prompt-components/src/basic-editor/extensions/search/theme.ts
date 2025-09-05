import { EditorView } from '@codemirror/view';
import { Prec } from '@codemirror/state';
export const theme = Prec.high(
  EditorView.theme({
    '& .cm-panels.cm-panels-top': {
      borderBottom: 'none',
      backgroundColor: 'transparent',
    },
    '& .cm-panels.cm-panels-bottom': {
      borderTop: 'none',
      backgroundColor: 'transparent',
      border: '1px solid rgba(28, 31, 35, .08)',
    },
    '& .cm-searchMatch': {
      outline: 'none',
    },

    '& .cm-custom-search': {
      position: 'absolute',
      top: 0,
      right: '10px',
      backgroundColor: '#f3f3f3',
      display: 'flex',
      padding: '4px',
      boxSizing: 'border-box',
      minWidth: '360px',
      maxWidth: 'calc(100% - 30px)',
      borderRadius: '4px',
      boxShadow: '0 2px 8px 2px rgba(0, 0, 0, 0.16)',
      overflow: 'hidden',
    },
    '& .cm-custom-search button': {
      borderWidth: 0,
    },
    '& .cm-custom-search input': {
      outline: 'none',
    },
    '& .cm-custom-search .cm-custom-search-drag': {
      width: '3px',
      height: '100%',
      position: 'absolute',
      top: '0px',
      left: '0px',
      backgroundColor: '#c8c8c8',
    },
    '& .cm-custom-search .cm-custom-search-drag:hover': {
      backgroundColor: '#0090f1',
      cursor: 'col-resize',
    },
    '& .cm-custom-text-field': {
      backgroundColor: 'inherit',
      padding: 0,
      border: 'none',
      color: '#444d56',
      fontSize: '12px',
      lineHeight: '16px',
      verticalAlign: 'middle',
      width: 'calc(100% - 58px)',
    },
    '& .cm-custom-search-panel-icon': {
      display: 'inline-flex',
      'align-items': 'center',
      'justify-content': 'center',
      padding: '4px',
      'border-radius': '2px',
      'background-color': 'transparent',
      'box-sizing': 'border-box',
      width: '20px',
      height: '20px',
      cursor: 'pointer',
    },
    '& .cm-custom-search-panel-icon:hover': {
      color: 'rgba(0, 100, 250, 1)',
      backgroundColor: 'rgba(46, 60, 56, .09)',
    },
    '& .cm-custom-search-panel-icon:active': {
      backgroundColor: 'rgba(46, 50, 56, .13)',
    },
    '& .cm-custom-search-panel-icon span': {
      display: 'inline-flex',
      lineHeight: '12px',
      fontSize: '12px',
    },
    '& .cm-custom-search-panel-icon-active': {
      color: 'rgba(0, 100, 250, 1)',
      backgroundColor: 'rgba(46, 50, 56, .05)',
    },
    '& .cm-custom-search-panel-icon-disabled': {
      color: 'rgba(28, 31, 35, .35)',
      backgroundColor: 'transparent',
    },
    '& .cm-custom-search-panel-icon-disabled:hover': {
      color: 'rgba(28, 31, 35, .35)',
      backgroundColor: 'transparent',
    },
    '& .cm-custom-search-panel-icon-disabled:active': {
      color: 'rgba(28, 31, 35, .35)',
      backgroundColor: 'transparent',
    },
    '& .cm-custom-search-panel-expand': {
      display: 'flex',
      padding: '4px',
    },
    '& .cm-custom-search-panel-content': {
      display: 'flex',
      flexDirection: 'column',
      flex: 1,
    },
    '& .cm-custom-search-panel-content-top': {
      display: 'flex',
      alignItems: 'center',
    },
    '& .cm-custom-search-action-wrapper': {
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'end',
    },
    '& .cm-custom-search-panel-content-input-wrapper': {
      display: 'flex',
      alignItems: 'center',
      flex: 1,
      position: 'relative',
      border: '1px solid #555',
      padding: '4px',
      minWidth: '170px',
      marginLeft: '4px',
      marginRight: '6px',
      borderRadius: '2px',
    },
    '& .cm-custom-search-panel-content-input-wrapper-focus': {
      borderColor: 'rgba(0, 100, 250, 1)',
    },
    '& .cm-custom-search-panel-content-input-wrapper .cm-custom-search-panel-icon':
      {
        position: 'absolute',
        top: '4px',
        right: '4px',
        padding: '2px',
        width: '16px',
        height: '16px !important',
      },
    '& .cm-panel.cm-custom-search [name=word]': {
      right: '22px',
    },
    '& .cm-panel.cm-custom-search [name=case]': {
      right: '40px',
    },
    '& .cm-panel.cm-custom-search [name=enter]': {
      right: '58px',
    },
    '& .cm-panel.cm-custom-search [name=previous]': {
      marginRight: '2px',
    },
    '& .cm-panel.cm-custom-search [name=next]': {
      marginRight: '2px',
    },
    '& .cm-panel.cm-custom-search [name=close-icon]': {
      marginRight: '2px',
    },
    '& .cm-panel.cm-custom-search [name=replace]': {
      marginRight: '2px',
    },
    '& .cm-panel.cm-custom-search  [name=expand]': {
      transform: 'rotate(0deg)',
    },
    '& .cm-panel.cm-custom-search  [name=expand].cm-custom-search-panel-icon-active':
      {
        transform: 'rotate(90deg)',
      },

    '& .cm-custom-search-match-count': {
      fontSize: '12px',
      width: '80px',
      overflow: 'hidden',
    },
    '& .cm-custom-search-panel-content-bottom': {
      marginRight: '104px',
      display: 'flex',
      alignItems: 'center',
    },
    '& .cm-custom-search-panel-content-bottom .cm-custom-search-panel-content-input-wrapper':
      {
        marginTop: '6px',
      },
  }),
);
