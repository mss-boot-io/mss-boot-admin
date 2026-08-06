const LANGUAGE_CACHE_VERSION = 1;

export const LANGUAGE_CACHE_KEY = 'mss-boot-admin:languages-public:v1';
export const LANGUAGE_CACHE_TTL_MS = 5 * 60 * 1000;

type LanguageCacheEntry = {
  version: number;
  expiresAt: number;
  data: API.Language[];
};

const getStorage = (): Storage | undefined => {
  if (typeof window === 'undefined') {
    return undefined;
  }

  try {
    return window.localStorage;
  } catch {
    return undefined;
  }
};

const removeCachedLanguages = () => {
  try {
    getStorage()?.removeItem(LANGUAGE_CACHE_KEY);
  } catch {
    // Storage can be disabled or full; caching must never block application startup.
  }
};

const isLanguageRecord = (value: unknown): value is API.Language => {
  if (!value || typeof value !== 'object') {
    return false;
  }

  const language = value as API.Language;
  return (
    typeof language.name === 'string' &&
    (language.defines === undefined || language.defines === null || Array.isArray(language.defines))
  );
};

const isLanguageCacheEntry = (value: unknown): value is LanguageCacheEntry => {
  if (!value || typeof value !== 'object') {
    return false;
  }

  const entry = value as LanguageCacheEntry;
  return (
    entry.version === LANGUAGE_CACHE_VERSION &&
    Number.isFinite(entry.expiresAt) &&
    Array.isArray(entry.data) &&
    entry.data.every(isLanguageRecord)
  );
};

export const getCachedLanguages = (): API.Language[] | undefined => {
  const storage = getStorage();
  if (!storage) {
    return undefined;
  }

  try {
    const raw = storage.getItem(LANGUAGE_CACHE_KEY);
    if (!raw) {
      return undefined;
    }

    const entry: unknown = JSON.parse(raw);
    if (!isLanguageCacheEntry(entry) || entry.expiresAt <= Date.now()) {
      removeCachedLanguages();
      return undefined;
    }

    return entry.data;
  } catch {
    removeCachedLanguages();
    return undefined;
  }
};

export const cacheLanguages = (data: API.Language[]) => {
  if (!data.every(isLanguageRecord)) {
    return;
  }

  try {
    getStorage()?.setItem(
      LANGUAGE_CACHE_KEY,
      JSON.stringify({
        version: LANGUAGE_CACHE_VERSION,
        expiresAt: Date.now() + LANGUAGE_CACHE_TTL_MS,
        data,
      } satisfies LanguageCacheEntry),
    );
  } catch {
    // Storage can be disabled or full; caching must never block application startup.
  }
};

export const invalidateLanguageCache = () => {
  removeCachedLanguages();
};
