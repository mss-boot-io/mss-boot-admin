import { createStyles } from 'antd-style';

export const useHeaderNoticeStyles = createStyles(({ css, token }) => ({
  trigger: css`
    align-items: center;
    appearance: none;
    background: transparent;
    border: 0;
    border-radius: ${token.borderRadiusLG}px;
    color: ${token.colorText};
    cursor: pointer;
    display: inline-flex;
    font-size: ${token.fontSizeLG}px;
    height: ${token.controlHeight}px;
    justify-content: center;
    line-height: 1;
    padding: 0;
    transition:
      background-color ${token.motionDurationMid},
      color ${token.motionDurationMid};
    width: ${token.controlHeight}px;

    &:hover,
    &[aria-expanded='true'] {
      background: ${token.colorFillTertiary};
      color: ${token.colorPrimary};
    }

    &:focus-visible {
      outline: ${token.lineWidthFocus}px solid ${token.colorPrimaryBorder};
      outline-offset: 2px;
    }
  `,
  panel: {
    width: '100%',
  },
  refreshAlert: {
    borderRadius: 0,
    borderInline: 0,
    borderTop: 0,
  },
  state: {
    alignItems: 'center',
    display: 'flex',
    justifyContent: 'center',
    minHeight: 248,
    padding: token.paddingLG,
  },
  loading: {
    alignItems: 'center',
    color: token.colorTextSecondary,
    display: 'flex',
    flexDirection: 'column',
    gap: token.marginSM,
  },
  error: {
    maxWidth: 320,
    width: '100%',
  },
  list: {
    listStyle: 'none',
    margin: 0,
    maxHeight: 360,
    overflowY: 'auto',
    padding: 0,
  },
  item: {
    borderBlockEnd: `${token.lineWidth}px solid ${token.colorBorderSecondary}`,
  },
  itemAction: css`
    align-items: flex-start;
    appearance: none;
    background: transparent;
    border: 0;
    color: ${token.colorText};
    cursor: pointer;
    display: flex;
    font: inherit;
    gap: ${token.marginSM}px;
    padding: ${token.paddingSM}px ${token.paddingLG}px;
    text-align: start;
    transition: background-color ${token.motionDurationMid};
    width: 100%;

    &:hover {
      background: ${token.colorFillQuaternary};
    }

    &:focus-visible {
      background: ${token.colorFillQuaternary};
      outline: ${token.lineWidthFocus}px solid ${token.colorPrimaryBorder};
      outline-offset: -${token.lineWidthFocus}px;
    }
  `,
  itemReadOnly: {
    alignItems: 'flex-start',
    display: 'flex',
    gap: token.marginSM,
    padding: `${token.paddingSM}px ${token.paddingLG}px`,
  },
  avatar: {
    flex: '0 0 auto',
    marginTop: token.marginXXS,
  },
  itemBody: {
    minWidth: 0,
    width: '100%',
  },
  titleRow: {
    alignItems: 'flex-start',
    display: 'flex',
    gap: token.marginXS,
    justifyContent: 'space-between',
  },
  title: {
    color: token.colorText,
    fontWeight: token.fontWeightStrong,
    minWidth: 0,
  },
  extra: {
    flex: '0 0 auto',
    marginInlineEnd: 0,
  },
  description: {
    color: token.colorTextSecondary,
    display: '-webkit-box',
    fontSize: token.fontSizeSM,
    lineHeight: token.lineHeightSM,
    marginTop: token.marginXXS,
    overflow: 'hidden',
    WebkitBoxOrient: 'vertical',
    WebkitLineClamp: 2,
  },
  datetime: {
    color: token.colorTextTertiary,
    fontSize: token.fontSizeSM,
    marginTop: token.marginXXS,
  },
  footer: {
    borderBlockStart: `${token.lineWidth}px solid ${token.colorBorderSecondary}`,
    padding: `${token.paddingXXS}px ${token.paddingXS}px`,
  },
}));
