import { ProSchemaValueEnumObj } from '@ant-design/pro-components';

interface IOptions {
  id?: string;
  name?: string;
}

export const toOptions = (data?: IOptions[]): ProSchemaValueEnumObj => {
  let options: ProSchemaValueEnumObj = {};
  if (!data) return options;
  data.forEach((item) => {
    options[item.id!] = {
      text: item.name,
      status: item.id,
    };
  });
  return options;
};

export const toOptionItems = (items?: API.OptionItem[]): ProSchemaValueEnumObj => {
  let options: ProSchemaValueEnumObj = {};
  if (!items) return options;
  items.forEach((item) => {
    options[item.value!] = {
      text: item.label || item.key,
      status: item.value,
      color: item.color,
    };
  });
  return options;
};
