export interface MockRequest {
  body: Record<string, string>;
}

export interface MockResponse {
  json: (body: unknown) => MockResponse;
  send: (body: unknown) => MockResponse;
  status: (code: number) => MockResponse;
}
