import { request } from '@umijs/max';

/**
 * Authenticated, paginated Language endpoint used by management lists.
 * Keep this separate from the generated public-language client used during
 * application bootstrap.
 */
export const getManagedLanguages = (
  params: API.getLanguagesParams,
) =>
  request<API.Page & { data?: API.Language[] }>('/admin/api/languages', {
    method: 'GET',
    params,
  });
