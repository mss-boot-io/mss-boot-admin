import type { RequestOptions } from '@@/plugin-request/request';

export function prepareAPIRequest(
  url: string,
  options: RequestOptions,
  apiURL = API_URL,
  token = localStorage.getItem('token'),
) {
  return {
    url: `${apiURL}${url}`,
    options: {
      ...options,
      // The deployed UI and API use different origins. OAuth browser-binding
      // cookies are HttpOnly, so every API request must opt into credentials.
      withCredentials: true,
      headers: token
        ? {
            ...options.headers,
            Authorization: `Bearer ${token}`,
          }
        : options.headers,
    },
  };
}
