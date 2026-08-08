import { expect, test, type APIRequestContext, type Locator, type Page } from '@playwright/test';

const API_BASE_URL = process.env.MSS_E2E_API_URL || 'http://localhost:8080/admin/api';
const THEME_MEDIA_TYPE = 'application/vnd.mss.theme.v1+json';
const THEME_KEYS = [
  'navTheme',
  'colorPrimary',
  'layout',
  'contentWidth',
  'fixedHeader',
  'fixSiderbar',
  'colorWeak',
] as const;

type ThemeKey = (typeof THEME_KEYS)[number];
type ThemeScope = 'application' | 'user';
type ThemeValue = string | boolean;
type ThemePatch = Partial<Record<ThemeKey, ThemeValue | null>>;
type ThemeResource = {
  overrides: Partial<Record<ThemeKey, ThemeValue>>;
  revision: string;
  etag: string;
};
type LoginSession = {
  expire: string;
  token: string;
};

const scopePath = (scope: ThemeScope) =>
  scope === 'application' ? 'app-configs/theme' : 'user-configs/theme';

const resourceETag = (scope: ThemeScope, revision: string) => `"theme-${scope}-${revision}"`;

async function readJSONOrThrow(response: Awaited<ReturnType<APIRequestContext['get']>>) {
  if (!response.ok()) {
    throw new Error(`${response.url()} returned ${response.status()}: ${await response.text()}`);
  }
  return response.json() as Promise<Record<string, any>>;
}

async function login(request: APIRequestContext): Promise<LoginSession> {
  const response = await request.post(`${API_BASE_URL}/user/login`, {
    data: { username: 'admin', password: '123456' },
  });
  const body = await readJSONOrThrow(response);
  expect(body.code).toBe(200);
  expect(body.token).toEqual(expect.any(String));
  return { token: body.token, expire: body.expire };
}

function themeHeaders(token: string, etag?: string) {
  return {
    Accept: THEME_MEDIA_TYPE,
    Authorization: `Bearer ${token}`,
    ...(etag ? { 'If-Match': etag } : {}),
  };
}

async function getThemeResource(
  request: APIRequestContext,
  token: string,
  scope: ThemeScope,
): Promise<ThemeResource> {
  const response = await request.get(`${API_BASE_URL}/${scopePath(scope)}`, {
    headers: themeHeaders(token),
  });
  const body = await readJSONOrThrow(response);
  expect(body._meta).toMatchObject({ v: 1, scope });
  expect(body._meta.revision).toMatch(/^\d+$/);

  const overrides: ThemeResource['overrides'] = {};
  for (const key of THEME_KEYS) {
    if (Object.prototype.hasOwnProperty.call(body, key)) {
      overrides[key] = body[key];
    }
  }
  return {
    overrides,
    revision: body._meta.revision,
    etag: response.headers().etag || resourceETag(scope, body._meta.revision),
  };
}

async function patchTheme(
  request: APIRequestContext,
  token: string,
  scope: ThemeScope,
  patch: ThemePatch,
): Promise<ThemeResource> {
  for (let attempt = 0; attempt < 3; attempt += 1) {
    const current = await getThemeResource(request, token, scope);
    const response = await request.put(`${API_BASE_URL}/${scopePath(scope)}`, {
      data: { data: patch },
      headers: themeHeaders(token, current.etag),
    });
    if (response.status() === 412) continue;
    const body = await readJSONOrThrow(response);
    expect(body._meta).toMatchObject({ v: 1, scope });
    return getThemeResource(request, token, scope);
  }
  throw new Error(`Could not update ${scope} theme after three revision conflicts`);
}

async function restoreTheme(
  request: APIRequestContext,
  token: string,
  scope: ThemeScope,
  backup: ThemeResource,
) {
  const patch = Object.fromEntries(
    THEME_KEYS.map((key) => [
      key,
      Object.prototype.hasOwnProperty.call(backup.overrides, key) ? backup.overrides[key] : null,
    ]),
  ) as ThemePatch;
  const restored = await patchTheme(request, token, scope, patch);
  expect(restored.overrides).toEqual(backup.overrides);
}

async function withRestoredThemes(
  request: APIRequestContext,
  token: string,
  run: () => Promise<void>,
) {
  const application = await getThemeResource(request, token, 'application');
  const user = await getThemeResource(request, token, 'user');
  try {
    await run();
  } finally {
    await restoreTheme(request, token, 'user', user);
    await restoreTheme(request, token, 'application', application);
  }
}

async function authenticatePage(page: Page, session: LoginSession, path: string) {
  await page.addInitScript(({ expire, token }) => {
    window.localStorage.setItem('token', token);
    window.localStorage.setItem('token.expire', expire);
    window.localStorage.setItem('autoLogin', 'true');
  }, session);
  await page.goto(path);
  await expect(page).not.toHaveURL(/\/user\/login/);
}

function themeFormItem(page: Page, label: string) {
  return page
    .locator('.ant-form-item')
    .filter({ has: page.getByText(label, { exact: true }) })
    .first();
}

async function waitForThemeEditor(page: Page, saveLabel: string) {
  await expect(page.getByRole('button', { name: saveLabel, exact: true })).toBeVisible({
    timeout: 15_000,
  });
  await expect(page.locator('form.ant-form').filter({ visible: true })).toHaveCount(1);
}

async function selectThemeOption(item: Locator, page: Page, option: string) {
  await item.locator('.ant-select-selector').click();
  await page
    .locator('.ant-select-dropdown:visible .ant-select-item-option-content')
    .getByText(option, { exact: true })
    .click();
}

async function expectNavigationTheme(page: Page, theme: 'light' | 'realDark') {
  const expectedClass = theme === 'light' ? 'ant-menu-light' : 'ant-menu-dark';
  await expect
    .poll(
      async () => {
        const menus = page.locator(
          '.ant-pro-sider .ant-menu:visible, .ant-layout-sider .ant-menu:visible',
        );
        const count = await menus.count();
        for (let index = 0; index < count; index += 1) {
          const className = (await menus.nth(index).getAttribute('class')) || '';
          if (className.includes(expectedClass)) return expectedClass;
        }
        return '';
      },
      { timeout: 10_000 },
    )
    .toBe(expectedClass);
}

async function selectControlContrastRatio(item: Locator) {
  return item.evaluate((element) => {
    type RGBA = [number, number, number, number];
    const parse = (value: string): RGBA => {
      const match = value.match(/rgba?\(([^)]+)\)/i);
      if (!match) throw new Error(`Unsupported browser color: ${value}`);
      const values = match[1]
        .split(/[,\s/]+/)
        .filter(Boolean)
        .map(Number);
      return [values[0], values[1], values[2], values[3] ?? 1];
    };
    const composite = (foreground: RGBA, background: RGBA): RGBA => {
      const alpha = foreground[3] + background[3] * (1 - foreground[3]);
      if (alpha === 0) return [255, 255, 255, 1];
      return [
        (foreground[0] * foreground[3] + background[0] * background[3] * (1 - foreground[3])) /
          alpha,
        (foreground[1] * foreground[3] + background[1] * background[3] * (1 - foreground[3])) /
          alpha,
        (foreground[2] * foreground[3] + background[2] * background[3] * (1 - foreground[3])) /
          alpha,
        alpha,
      ];
    };
    const luminance = ([red, green, blue]: RGBA) => {
      const channel = (value: number) => {
        const normalized = value / 255;
        return normalized <= 0.04045 ? normalized / 12.92 : ((normalized + 0.055) / 1.055) ** 2.4;
      };
      return channel(red) * 0.2126 + channel(green) * 0.7152 + channel(blue) * 0.0722;
    };

    const selector = element.querySelector('.ant-select-selector');
    const selectedValue = element.querySelector('.ant-select-selection-item');
    if (!selector || !selectedValue) throw new Error('Theme select control is not rendered');
    const background = parse(getComputedStyle(selector).backgroundColor);
    if (background[3] !== 1) {
      throw new Error(`Theme select background must be opaque: ${background.join(',')}`);
    }
    const computedColor = getComputedStyle(selectedValue).color;
    const foreground = composite(parse(computedColor), background);
    const lighter = Math.max(luminance(foreground), luminance(background));
    const darker = Math.min(luminance(foreground), luminance(background));
    return {
      background,
      computedColor,
      foreground,
      ratio: (lighter + 0.05) / (darker + 0.05),
    };
  });
}

test.describe('Layered theme settings', () => {
  test.describe.configure({ mode: 'serial', timeout: 90_000 });

  test.describe('English desktop', () => {
    test.use({ locale: 'en-US', viewport: { width: 1440, height: 1000 } });

    test('application scope saves and resets the runtime theme with accessible focus and contrast', async ({
      page,
    }) => {
      const session = await login(page.request);
      await withRestoredThemes(page.request, session.token, async () => {
        await patchTheme(page.request, session.token, 'user', { navTheme: null });
        await patchTheme(page.request, session.token, 'application', {
          navTheme: 'realDark',
          fixedHeader: false,
        });

        await authenticatePage(page, session, '/app-config?key=theme');
        await waitForThemeEditor(page, 'Save theme');
        await expect(page.getByText('Application Settings', { exact: true }).first()).toBeVisible();

        const navigation = themeFormItem(page, 'Navigation Theme');
        const layout = themeFormItem(page, 'Layout');
        await expect(navigation.locator('.ant-tag')).toHaveText('Application override');
        await expect(layout.locator('.ant-tag')).toHaveText('Code default');

        const combobox = navigation.getByRole('combobox');
        await combobox.focus();
        await expect(combobox).toBeFocused();
        await page.keyboard.press('Enter');
        await expect(
          page.locator('.ant-select-dropdown:visible').getByText('Dark', { exact: true }),
        ).toBeVisible();
        await page.keyboard.press('Escape');

        await expectNavigationTheme(page, 'realDark');
        const darkContrast = await selectControlContrastRatio(navigation);
        expect(darkContrast.ratio, JSON.stringify(darkContrast)).toBeGreaterThanOrEqual(4.5);

        await selectThemeOption(navigation, page, 'Light');
        const save = page.getByRole('button', { name: 'Save theme', exact: true });
        await expect(save).toBeEnabled();
        const savedResponse = page.waitForResponse(
          (response) =>
            response.request().method() === 'PUT' &&
            response.url().includes('/admin/api/app-configs/theme'),
        );
        await save.click();
        const saved = await savedResponse;
        expect(saved.ok()).toBeTruthy();
        expect(saved.request().headers().accept).toBe(THEME_MEDIA_TYPE);
        expect(saved.request().headers()['if-match']).toMatch(/^"theme-application-\d+"$/);
        await expectNavigationTheme(page, 'light');
        await page.keyboard.press('Escape');
        await expect(navigation.getByRole('combobox')).toBeEnabled();
        await expect(page.locator('.ant-select-dropdown:visible')).toHaveCount(0);
        const lightContrast = await selectControlContrastRatio(navigation);
        expect(lightContrast.ratio, JSON.stringify(lightContrast)).toBeGreaterThanOrEqual(4.5);

        const resetResponse = page.waitForResponse(
          (response) =>
            response.request().method() === 'PUT' &&
            response.url().includes('/admin/api/app-configs/theme'),
        );
        await navigation.getByRole('button', { name: 'Restore inherited', exact: true }).click();
        expect((await resetResponse).ok()).toBeTruthy();
        await expect(navigation.locator('.ant-tag')).toHaveText('Code default');
        await expectNavigationTheme(page, 'realDark');
      });
    });

    test('personal scope exposes field provenance, per-field reset, and confirmed whole reset', async ({
      page,
    }) => {
      const session = await login(page.request);
      await withRestoredThemes(page.request, session.token, async () => {
        await patchTheme(page.request, session.token, 'application', {
          navTheme: 'light',
          colorPrimary: null,
          contentWidth: 'Fixed',
          fixedHeader: true,
        });
        await patchTheme(page.request, session.token, 'user', {
          navTheme: 'realDark',
          colorPrimary: '#722ed1',
          fixedHeader: false,
        });

        await authenticatePage(page, session, '/account/settings?key=theme');
        await waitForThemeEditor(page, 'Save theme');
        await expect(page.getByText('Personal Settings', { exact: true }).first()).toBeVisible();

        const navigation = themeFormItem(page, 'Navigation Theme');
        const contentWidth = themeFormItem(page, 'Content Width');
        const layout = themeFormItem(page, 'Layout');
        await expect(navigation.locator('.ant-tag')).toHaveText('Personal override');
        await expect(contentWidth.locator('.ant-tag')).toHaveText('Application override');
        await expect(layout.locator('.ant-tag')).toHaveText('Code default');
        await expectNavigationTheme(page, 'realDark');

        const fieldReset = page.waitForResponse(
          (response) =>
            response.request().method() === 'PUT' &&
            response.url().includes('/admin/api/user-configs/theme'),
        );
        await navigation.getByRole('button', { name: 'Restore inherited', exact: true }).click();
        expect((await fieldReset).ok()).toBeTruthy();
        await expect(navigation.locator('.ant-tag')).toHaveText('Application override');
        await expectNavigationTheme(page, 'light');

        const resetAll = page.getByRole('button', {
          name: 'Restore all inherited settings',
          exact: true,
        });
        await resetAll.click();
        await expect(
          page.getByText('Restore every theme setting to its inherited value?', { exact: true }),
        ).toBeVisible();
        const wholeReset = page.waitForResponse(
          (response) =>
            response.request().method() === 'DELETE' &&
            response.url().includes('/admin/api/user-configs/theme'),
        );
        await page.getByRole('button', { name: 'OK', exact: true }).click();
        expect((await wholeReset).ok()).toBeTruthy();
        await expect(resetAll).toBeDisabled();
        await expect(navigation.locator('.ant-tag')).toHaveText('Application override');
        await expect(themeFormItem(page, 'Primary Color').locator('.ant-tag')).toHaveText(
          'Code default',
        );
        const user = await getThemeResource(page.request, session.token, 'user');
        expect(user.overrides).toEqual({});
      });
    });

    test('a versioned snapshot provides a stable warm first paint before authoritative reads finish', async ({
      page,
    }) => {
      const session = await login(page.request);
      await withRestoredThemes(page.request, session.token, async () => {
        await patchTheme(page.request, session.token, 'application', {
          navTheme: 'realDark',
          colorPrimary: '#1677ff',
        });
        await patchTheme(page.request, session.token, 'user', {
          navTheme: 'light',
          colorPrimary: '#13a8a8',
        });

        await authenticatePage(page, session, '/account/settings?key=theme');
        await waitForThemeEditor(page, 'Save theme');
        await expect
          .poll(() =>
            page.evaluate(() => {
              const sessionID = window.localStorage.getItem('mss.auth.theme-session.v1');
              return Boolean(
                window.localStorage.getItem('mss.theme.application.v1') &&
                  sessionID &&
                  window.localStorage.getItem(`mss.theme.user.v1:${sessionID}`),
              );
            }),
          )
          .toBe(true);

        await page.addInitScript(() => {
          const timeline: string[] = [];
          Object.assign(window, { __mssThemeTimeline: timeline });
          const record = () => {
            const theme = document.documentElement?.dataset.mssTheme;
            if (theme && timeline[timeline.length - 1] !== theme) timeline.push(theme);
          };
          new MutationObserver(record).observe(document, {
            attributes: true,
            attributeFilter: ['data-mss-theme'],
            subtree: true,
          });
          queueMicrotask(record);
        });
        await page.route(/\/admin\/api\/(app|user)-configs\/profile/, async (route) => {
          await new Promise<void>((resolve) => {
            setTimeout(resolve, 1_200);
          });
          await route.continue();
        });

        await page.reload({ waitUntil: 'domcontentloaded' });
        await expect
          .poll(() => page.locator('html').getAttribute('data-mss-theme'), { timeout: 1_000 })
          .toBe('light');
        await expect
          .poll(() =>
            page.evaluate(() =>
              document.documentElement.style.getPropertyValue('--mss-theme-color-primary'),
            ),
          )
          .toBe('#13a8a8');

        await waitForThemeEditor(page, 'Save theme');
        const timeline = await page.evaluate(
          () => (window as Window & { __mssThemeTimeline?: string[] }).__mssThemeTimeline || [],
        );
        expect(timeline).toEqual(['light']);
        await expect(page.locator('html')).toHaveCSS('color-scheme', 'light');
      });
    });
  });

  test.describe('Chinese mobile', () => {
    test.use({
      locale: 'zh-CN',
      viewport: { width: 390, height: 844 },
      isMobile: true,
      hasTouch: true,
    });

    test('personal editor keeps localized provenance, equivalent controls, and a non-overflowing layout', async ({
      page,
    }) => {
      const session = await login(page.request);
      await withRestoredThemes(page.request, session.token, async () => {
        await patchTheme(page.request, session.token, 'application', { contentWidth: 'Fixed' });
        await patchTheme(page.request, session.token, 'user', { navTheme: 'light' });

        await authenticatePage(page, session, '/account/settings?key=theme');
        await waitForThemeEditor(page, '保存主题');
        await expect(page.getByText('个人设置', { exact: true }).first()).toBeVisible();
        await expect(themeFormItem(page, '导航主题').locator('.ant-tag')).toHaveText('个人级覆盖');
        await expect(themeFormItem(page, '内容宽度').locator('.ant-tag')).toHaveText('应用级覆盖');
        await expect(themeFormItem(page, '布局').locator('.ant-tag')).toHaveText('代码默认值');

        for (const label of [
          '导航主题',
          '主题色',
          '布局',
          '内容宽度',
          '固定头部',
          '固定侧边栏',
          '色弱模式',
        ]) {
          await expect(themeFormItem(page, label)).toBeVisible();
        }
        await expect(page.getByRole('button', { name: '全部恢复继承', exact: true })).toBeVisible();

        const navigation = themeFormItem(page, '导航主题');
        const combobox = navigation.getByRole('combobox');
        await combobox.focus();
        await expect(combobox).toBeFocused();
        await page.keyboard.press('Enter');
        await expect(
          page.locator('.ant-select-dropdown:visible').getByText('浅色', { exact: true }),
        ).toBeVisible();
        await page.keyboard.press('Escape');

        const widths = await page.evaluate(() => ({
          body: document.body.scrollWidth,
          document: document.documentElement.scrollWidth,
          viewport: window.innerWidth,
          form: document.querySelector('form.ant-form')?.getBoundingClientRect().width || 0,
        }));
        expect(widths.body).toBeLessThanOrEqual(widths.viewport + 1);
        expect(widths.document).toBeLessThanOrEqual(widths.viewport + 1);
        expect(widths.form).toBeGreaterThan(0);
        expect(widths.form).toBeLessThanOrEqual(widths.viewport);
      });
    });
  });
});
