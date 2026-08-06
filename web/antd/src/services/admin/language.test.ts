import { request } from '@umijs/max';
import {
  deleteLanguagesId,
  getCachedLanguages,
  postLanguages,
  putLanguagesId,
} from './language';
import {
  cacheLanguages,
  LANGUAGE_CACHE_KEY,
  LANGUAGE_CACHE_TTL_MS,
} from './languageCache';

jest.mock('@umijs/max', () => ({
  request: jest.fn(),
}));

const mockRequest = request as jest.Mock;
const languageWithEmptyDefines: API.Language = {
  id: 'fr-fr',
  name: 'fr-FR',
  defines: [],
};
const freshLanguage: API.Language = {
  id: 'en-us',
  name: 'en-US',
  defines: [{ group: 'pages', key: 'title', value: 'Title' }],
};

describe('public language bundle cache', () => {
  const storage = new Map<string, string>();

  beforeEach(() => {
    jest.clearAllMocks();
    storage.clear();
    jest.spyOn(Date, 'now').mockReturnValue(1000);
    (localStorage.getItem as jest.Mock).mockImplementation((key: string) => storage.get(key) ?? null);
    (localStorage.setItem as jest.Mock).mockImplementation((key: string, value: string) => {
      storage.set(key, value);
    });
    (localStorage.removeItem as jest.Mock).mockImplementation((key: string) => {
      storage.delete(key);
    });
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  it('uses a fresh cache without calling the public API and preserves empty definitions', async () => {
    cacheLanguages([languageWithEmptyDefines]);

    const result = await getCachedLanguages();

    expect(result.data).toEqual([languageWithEmptyDefines]);
    expect(result.data?.[0].defines).toEqual([]);
    expect(mockRequest).not.toHaveBeenCalled();
  });

  it.each([
    ['expired', JSON.stringify({ version: 1, expiresAt: 999, data: [languageWithEmptyDefines] })],
    ['corrupt', '{not-json'],
  ])('falls back to the public API when the cache is %s', async (_, cachedValue) => {
    storage.set(LANGUAGE_CACHE_KEY, cachedValue);
    mockRequest.mockResolvedValue({ data: [freshLanguage] });

    const result = await getCachedLanguages();

    expect(result.data).toEqual([freshLanguage]);
    expect(mockRequest).toHaveBeenCalledWith('/admin/api/languages/public', {
      method: 'GET',
      params: { pageSize: 999 },
    });
    expect(localStorage.removeItem).toHaveBeenCalledWith(LANGUAGE_CACHE_KEY);
  });

  it('normalizes and caches the raw array returned by the public endpoint', async () => {
    mockRequest.mockResolvedValue([freshLanguage]);

    expect((await getCachedLanguages()).data).toEqual([freshLanguage]);
    expect(mockRequest).toHaveBeenCalledTimes(1);

    expect((await getCachedLanguages()).data).toEqual([freshLanguage]);
    expect(mockRequest).toHaveBeenCalledTimes(1);
  });

  it('invalidates the cache after every successful language mutation', async () => {
    mockRequest.mockResolvedValue({});

    const mutations = [
      () => postLanguages(languageWithEmptyDefines),
      () => putLanguagesId({ id: languageWithEmptyDefines.id! }, languageWithEmptyDefines),
      () => deleteLanguagesId({ id: languageWithEmptyDefines.id! }),
    ];

    for (const mutate of mutations) {
      cacheLanguages([languageWithEmptyDefines]);
      expect(storage.has(LANGUAGE_CACHE_KEY)).toBe(true);

      await mutate();

      expect(storage.has(LANGUAGE_CACHE_KEY)).toBe(false);
    }

    expect(localStorage.removeItem).toHaveBeenCalledTimes(3);
  });

  it('uses a five-minute TTL', () => {
    cacheLanguages([languageWithEmptyDefines]);

    expect(JSON.parse(storage.get(LANGUAGE_CACHE_KEY)!)).toMatchObject({
      expiresAt: 1000 + LANGUAGE_CACHE_TTL_MS,
    });
  });
});
