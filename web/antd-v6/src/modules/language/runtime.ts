import enUS from 'antd/locale/en_US';
import zhCN from 'antd/locale/zh_CN';
import type { LanguageProfile, SupportedRuntimeLocale } from './contract';

interface LocaleExtra {
  antd: typeof enUS;
  momentLocale: string;
}

export type RuntimeLocaleRegistrar = (
  locale: SupportedRuntimeLocale,
  messages: Readonly<Record<string, string>>,
  extra: LocaleExtra,
) => void;

const extras: Readonly<Record<SupportedRuntimeLocale, LocaleExtra>> = {
  'en-US': { antd: enUS, momentLocale: 'en' },
  'zh-CN': { antd: zhCN, momentLocale: 'zh-cn' },
};

/**
 * Merge authoritative dynamic messages only into the two statically complete
 * catalogs compiled into this application. A full reload reconstructs the
 * static baseline before applying the latest profile, so deleted keys cannot
 * linger from Umi's merge-only addLocale API.
 */
export function registerSupportedLanguageProfile(
  profile: LanguageProfile,
  register: RuntimeLocaleRegistrar,
): void {
  for (const locale of ['en-US', 'zh-CN'] as const) {
    const messages = profile[locale];
    if (messages) register(locale, messages, extras[locale]);
  }
}
