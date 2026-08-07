import type { RequestOptions } from '@@/plugin-request/request';
import { getAuthToken } from '@/utils/authStorage';

export function prepareAPIRequest(
  url: string,
  options: RequestOptions,
  apiURL = API_URL,
  token = getAuthToken(),
) {
  const { skipAuthToken, ...requestOptions } = options as RequestOptions & {
    skipAuthToken?: boolean;
  };
  return {
    url: `${apiURL}${url}`,
    options: {
      ...requestOptions,
      // The deployed UI and API use different origins. OAuth browser-binding
      // cookies are HttpOnly, so every API request must opt into credentials.
      withCredentials: true,
      headers:
        token && !skipAuthToken
          ? {
              ...requestOptions.headers,
              Authorization: `Bearer ${token}`,
            }
          : requestOptions.headers,
    },
  };
}
