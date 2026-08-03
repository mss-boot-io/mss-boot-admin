import { DataNode } from 'antd/es/tree';
// @ts-ignore
import { IntlShape } from 'react-intl';

export const menuTransferTree = (intl: IntlShape, data: API.Menu[]): DataNode[] => {
  // @ts-ignore
  return data.map((item) => {
    const menuId = item.name?.startsWith('menu.') ? item.name : `menu.${item.name}`;
    return {
      title: intl.formatMessage({ id: menuId }),
      value: item.id,
      // @ts-ignore
      children: item.children ? menuTransferTree(intl, item.children) : null,
    };
  });
};
