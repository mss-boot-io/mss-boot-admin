// @ts-ignore
/* eslint-disable */
import { ProColumns } from '@ant-design/pro-components';

declare namespace API {
  type listVirtualModelsParams = {
    key?: string;
  } & {
    [key: string]: any;
  } & API.Page;

  type getVirtualModelParams = {
    key?: string;
    id?: string;
  } & {
    [key: string]: any;
  };

  type createVirtualModelParams = {
    key?: string;
  } & {
    [key: string]: any;
  };

  type updateVirtualModelParams = {
    key?: string;
    id?: string;
  } & {
    [key: string]: any;
  };

  type deleteVirtualModelParams = {
    key?: string;
    id?: string;
  } & {
    [key: string]: any;
  };

  type getVirtualDocumentationParams = {
    key?: string;
  } & {
    [key: string]: any;
  };

  type documentation = {
    name: string;
    columns: ProColumns<{ [key: string]: any }>[];
  };
}
