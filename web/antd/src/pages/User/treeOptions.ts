export type TreeOptionRecord = {
  id?: string;
  name?: string;
  children?: TreeOptionRecord[];
};

export type TreeOptionNode = {
  key: string;
  title: string;
  value: string;
  disabled: boolean;
  children?: TreeOptionNode[];
};

export const buildTreeOptions = (data: TreeOptionRecord[], self?: string): TreeOptionNode[] =>
  data.map((item) => ({
    key: item.id || '',
    title: item.name || '',
    value: item.id || '',
    disabled: item.id === self,
    children: item.children ? buildTreeOptions(item.children, self) : undefined,
  }));

export const findTreeOptionLabel = (data: TreeOptionNode[], key: string): string => {
  for (const item of data) {
    if (item.value === key) {
      return item.title;
    }
    if (item.children) {
      const nestedLabel = findTreeOptionLabel(item.children, key);
      if (nestedLabel) {
        return nestedLabel;
      }
    }
  }
  return '';
};
