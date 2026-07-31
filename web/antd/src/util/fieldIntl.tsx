// @ts-ignore
import { IntlShape } from 'react-intl';

export const fieldIntl = (intl: IntlShape, name: string): string => {
  return intl.formatMessage({ id: `pages.fields.${name}`, defaultMessage: name });
};
