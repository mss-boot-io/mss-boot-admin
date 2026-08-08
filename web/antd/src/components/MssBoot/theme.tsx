import {
  createThemeSettingsAdapter,
  isThemeRevisionConflictError,
  type ThemeSettingsAdapter,
  type ThemeSettingsScope,
} from '@/services/admin/themeSettings';
import {
  areThemeOverridesEqual,
  CODE_THEME_DEFAULTS,
  getThemeOverrides,
  getVerifiedThemeAuthSessionId,
  hasThemeOverride,
  normalizeThemeOverrides,
  reconcileThemeScopeResource,
  resolveThemeSettings,
  THEME_SETTING_KEYS,
  type ThemeOverrides,
  type ThemePatch,
  type ThemeScopeResource,
  type ThemeSettingKey,
  type ThemeSettings,
} from '@/utils/themeSettings';
import { getThemeAuthSessionId } from '@/utils/themeSession';
import { publishThemeScopeResource } from '@/utils/themeSync';
import { fieldIntl } from '@/util/fieldIntl';
import { useIntl, useModel } from '@umijs/max';
import {
  Alert,
  Button,
  ColorPicker,
  Form,
  Popconfirm,
  Select,
  Skeleton,
  Space,
  Switch,
  Tag,
  message,
} from 'antd';
import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';

export type ThemeProps = {
  scope: ThemeSettingsScope;
  adapter?: ThemeSettingsAdapter;
  onApplied?: (settings: ThemeSettings, overrides: ThemeOverrides) => void;
};

const APPLICATION_CONFIG_WRITE_PERMISSION = '/app-config/control';
const THEME_LOAD_MAX_ATTEMPTS = 3;

const getErrorStatus = (value: unknown): number | undefined => {
  if (typeof value !== 'object' || value === null) return undefined;
  const error = value as {
    status?: unknown;
    response?: { status?: unknown };
    info?: { errorCode?: unknown };
  };
  const status = error.response?.status ?? error.status ?? error.info?.errorCode;
  if (typeof status === 'number') return status;
  if (typeof status === 'string' && /^\d+$/.test(status)) return Number(status);
  return undefined;
};

const Theme: React.FC<ThemeProps> = ({ scope, adapter, onApplied }) => {
  const intl = useIntl();
  const { initialState, setInitialState } = useModel('@@initialState');
  const stateRef = useRef(initialState);
  const [form] = Form.useForm<ThemeSettings>();
  const [messageApi, messageContextHolder] = message.useMessage();
  const dataSource = useMemo(() => adapter || createThemeSettingsAdapter(scope), [adapter, scope]);
  const [resource, setResource] = useState<ThemeScopeResource>({
    scope,
    revision: '0',
    overrides: {},
    versioned: false,
  });
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [dirtyKeys, setDirtyKeys] = useState<Set<ThemeSettingKey>>(() => new Set());
  const [conflict, setConflict] = useState<ThemeScopeResource>();
  const [resettingKey, setResettingKey] = useState<ThemeSettingKey>();
  const [resettingAll, setResettingAll] = useState(false);
  const [error, setError] = useState<Error>();
  const [hasLoadedCanonicalResource, setHasLoadedCanonicalResource] = useState(false);
  const [identityChanged, setIdentityChanged] = useState(false);
  const mutationInFlightRef = useRef(false);
  const overrides = resource.overrides;
  const dirty = dirtyKeys.size > 0;
  const mutating = saving || resettingAll || resettingKey !== undefined;
  const verifiedAuthSessionId = getVerifiedThemeAuthSessionId(
    initialState,
    getThemeAuthSessionId(),
  );
  const identityMismatch =
    Boolean(initialState?.currentUser) && (!verifiedAuthSessionId || identityChanged);
  const permissions = initialState?.currentUser?.permissions;
  const hasMutationPermission =
    scope === 'user'
      ? Boolean(initialState?.currentUser && verifiedAuthSessionId)
      : Boolean(initialState?.currentUser && verifiedAuthSessionId) &&
        (initialState?.currentUser?.role?.root === true ||
          (!!permissions &&
            Object.prototype.hasOwnProperty.call(
              permissions,
              APPLICATION_CONFIG_WRITE_PERMISSION,
            )));
  const canMutate = hasMutationPermission && hasLoadedCanonicalResource && !identityChanged;

  const applicationOverrides = useMemo(
    () => getThemeOverrides(initialState?.appConfig),
    [initialState?.appConfig],
  );
  const inherited = useMemo(
    () =>
      scope === 'application'
        ? { ...CODE_THEME_DEFAULTS }
        : resolveThemeSettings(CODE_THEME_DEFAULTS, applicationOverrides),
    [applicationOverrides, scope],
  );
  const inheritedRef = useRef(inherited);
  const effective = useMemo(
    () => resolveThemeSettings(inherited, undefined, overrides),
    [inherited, overrides],
  );
  const runtimeResource = initialState?.themeRuntime?.layers?.[scope];
  const degraded = initialState?.themeRuntime?.degradedScopes?.includes(scope) === true;
  const permissionDenied = scope === 'application' && getErrorStatus(error) === 403;

  useEffect(() => {
    stateRef.current = initialState;
  }, [initialState]);

  useEffect(() => {
    inheritedRef.current = inherited;
  }, [inherited]);

  useEffect(() => {
    if (verifiedAuthSessionId) {
      setIdentityChanged(false);
    }
  }, [verifiedAuthSessionId]);

  const hasCurrentMutationIdentity = useCallback(() => {
    if (!canMutate) return false;
    const currentSessionId = getVerifiedThemeAuthSessionId(
      stateRef.current,
      getThemeAuthSessionId(),
    );
    if (!currentSessionId || currentSessionId !== verifiedAuthSessionId) {
      setIdentityChanged(true);
      return false;
    }
    return true;
  }, [canMutate, verifiedAuthSessionId]);

  const applyCanonicalResource = useCallback(
    (nextResource: ThemeScopeResource, publish = true) => {
      const authSessionId = scope === 'user' ? verifiedAuthSessionId : undefined;
      if (scope === 'user' && (!authSessionId || getThemeAuthSessionId() !== authSessionId)) {
        return false;
      }
      const preview = reconcileThemeScopeResource(stateRef.current || {}, nextResource, {
        allowLegacyReplace: !nextResource.versioned,
        authSessionId,
        authoritative: true,
      });
      if (preview.status !== 'applied') {
        return false;
      }
      const nextEffective = resolveThemeSettings(
        inheritedRef.current,
        undefined,
        nextResource.overrides,
      );
      stateRef.current = preview.state as typeof stateRef.current;
      setInitialState((previous) => {
        const result = reconcileThemeScopeResource(previous || {}, nextResource, {
          allowLegacyReplace: !nextResource.versioned,
          authSessionId,
          authoritative: true,
        });
        stateRef.current = result.state as typeof stateRef.current;
        return result.state as typeof previous;
      });
      if (publish) {
        publishThemeScopeResource(nextResource, authSessionId);
      }
      onApplied?.(nextEffective, nextResource.overrides);
      return true;
    },
    [onApplied, scope, setInitialState, verifiedAuthSessionId],
  );

  const acceptCanonicalResource = useCallback(
    (nextResource: ThemeScopeResource, publish = true) => {
      if (!applyCanonicalResource(nextResource, publish)) {
        return false;
      }
      setResource(nextResource);
      return true;
    },
    [applyCanonicalResource],
  );

  const load = useCallback(async () => {
    setLoading(true);
    setError(undefined);
    if (scope === 'user' && !verifiedAuthSessionId) {
      setLoading(false);
      return;
    }
    try {
      let accepted = false;
      for (let attempt = 0; attempt < THEME_LOAD_MAX_ATTEMPTS; attempt += 1) {
        const nextResource = await dataSource.load();
        if (acceptCanonicalResource(nextResource, false)) {
          accepted = true;
          break;
        }
      }
      if (!accepted) {
        throw new Error('Theme settings changed repeatedly while loading');
      }
      // Legacy responses are still valid mutation bases. This flag means the
      // current scope was read successfully, not that revision metadata exists.
      setHasLoadedCanonicalResource(true);
      setDirtyKeys(new Set());
      setConflict(undefined);
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError : new Error(String(loadError)));
    } finally {
      setLoading(false);
    }
  }, [acceptCanonicalResource, dataSource, scope, verifiedAuthSessionId]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    if (loading) {
      return;
    }
    const untouchedValues: Partial<ThemeSettings> = {};
    THEME_SETTING_KEYS.forEach((key) => {
      if (!dirtyKeys.has(key)) {
        Object.assign(untouchedValues, { [key]: effective[key] });
      }
    });
    form.setFieldsValue(untouchedValues);
  }, [dirtyKeys, effective, form, loading]);

  useEffect(() => {
    if (
      loading ||
      !runtimeResource ||
      (runtimeResource.revision === resource.revision &&
        areThemeOverridesEqual(runtimeResource.overrides, resource.overrides))
    ) {
      return;
    }
    setResource(runtimeResource);
    if (dirty) {
      setConflict(runtimeResource);
    } else {
      setConflict(undefined);
    }
  }, [dirty, loading, resource, runtimeResource]);

  const handleRevisionConflict = useCallback(
    async (conflictError: unknown) => {
      if (!isThemeRevisionConflictError(conflictError)) {
        return false;
      }
      try {
        const latest = conflictError.current || (await dataSource.load());
        if (!acceptCanonicalResource(latest, false)) return true;
        setConflict(latest);
      } catch (loadError) {
        setError(loadError instanceof Error ? loadError : new Error(String(loadError)));
      }
      return true;
    },
    [acceptCanonicalResource, dataSource],
  );

  const sourceLabel = (key: ThemeSettingKey) => {
    if (hasThemeOverride(overrides, key)) {
      return intl.formatMessage({
        id:
          scope === 'application'
            ? 'pages.theme.settings.source.application'
            : 'pages.theme.settings.source.user',
        defaultMessage: scope === 'application' ? 'Application override' : 'Personal override',
      });
    }
    if (scope === 'user' && hasThemeOverride(applicationOverrides, key)) {
      return intl.formatMessage({
        id: 'pages.theme.settings.source.application',
        defaultMessage: 'Application override',
      });
    }
    return intl.formatMessage({
      id: 'pages.theme.settings.source.code',
      defaultMessage: 'Code default',
    });
  };

  const resetKey = async (key: ThemeSettingKey) => {
    if (mutationInFlightRef.current || !hasCurrentMutationIdentity()) return;
    mutationInFlightRef.current = true;
    setResettingKey(key);
    setError(undefined);
    try {
      const nextResource = await dataSource.reset([key], resource);
      if (!acceptCanonicalResource(nextResource)) return;
      setDirtyKeys((previous) => {
        const next = new Set(previous);
        next.delete(key);
        return next;
      });
      setConflict(undefined);
      form.setFieldValue(
        key,
        resolveThemeSettings(inherited, undefined, nextResource.overrides)[key],
      );
      messageApi.success(
        intl.formatMessage({
          id: 'pages.theme.settings.reset.success',
          defaultMessage: 'Inherited setting restored',
        }),
      );
    } catch (resetError) {
      if (!(await handleRevisionConflict(resetError))) {
        setError(resetError instanceof Error ? resetError : new Error(String(resetError)));
      }
    } finally {
      mutationInFlightRef.current = false;
      setResettingKey(undefined);
    }
  };

  const resetAll = async () => {
    if (mutationInFlightRef.current || !hasCurrentMutationIdentity()) return;
    mutationInFlightRef.current = true;
    setResettingAll(true);
    setError(undefined);
    try {
      const nextResource = await dataSource.reset(undefined, resource);
      if (!acceptCanonicalResource(nextResource)) return;
      form.setFieldsValue(resolveThemeSettings(inherited, undefined, nextResource.overrides));
      setDirtyKeys(new Set());
      setConflict(undefined);
      messageApi.success(
        intl.formatMessage({
          id: 'pages.theme.settings.resetAll.success',
          defaultMessage: 'All inherited settings restored',
        }),
      );
    } catch (resetError) {
      if (!(await handleRevisionConflict(resetError))) {
        setError(resetError instanceof Error ? resetError : new Error(String(resetError)));
      }
    } finally {
      mutationInFlightRef.current = false;
      setResettingAll(false);
    }
  };

  const submit = async (values: ThemeSettings) => {
    if (mutationInFlightRef.current || !hasCurrentMutationIdentity()) return;
    mutationInFlightRef.current = true;
    const submitted = normalizeThemeOverrides(values);
    const patch: ThemePatch = {};

    dirtyKeys.forEach((key) => {
      const value = submitted[key];
      if (value === undefined) {
        return;
      }
      const alreadyOverridden = hasThemeOverride(overrides, key);
      if (!alreadyOverridden && value === inherited[key]) {
        return;
      }
      if (!alreadyOverridden || overrides[key] !== value) {
        Object.assign(patch, { [key]: value });
      }
    });

    if (Object.keys(patch).length === 0) {
      mutationInFlightRef.current = false;
      setDirtyKeys(new Set());
      return;
    }

    setSaving(true);
    setError(undefined);
    try {
      const nextResource = await dataSource.patch(patch, resource);
      if (!acceptCanonicalResource(nextResource)) return;
      setDirtyKeys(new Set());
      setConflict(undefined);
      messageApi.success(
        intl.formatMessage({
          id: 'pages.message.edit.success',
          defaultMessage: 'Update Success!',
        }),
      );
    } catch (saveError) {
      if (!(await handleRevisionConflict(saveError))) {
        setError(saveError instanceof Error ? saveError : new Error(String(saveError)));
      }
    } finally {
      mutationInFlightRef.current = false;
      setSaving(false);
    }
  };

  const label = (key: ThemeSettingKey, title: React.ReactNode) => (
    <Space size={4} wrap>
      <span>{title}</span>
      <Tag color={hasThemeOverride(overrides, key) ? 'blue' : 'default'}>{sourceLabel(key)}</Tag>
      {hasThemeOverride(overrides, key) && (
        <Button
          disabled={!canMutate || mutating || Boolean(conflict)}
          htmlType="button"
          loading={resettingKey === key}
          onClick={() => void resetKey(key)}
          size="small"
          type="link"
        >
          {intl.formatMessage({
            id: 'pages.theme.settings.inherit',
            defaultMessage: 'Restore inherited',
          })}
        </Button>
      )}
    </Space>
  );

  const reloadConflict = () => {
    if (!conflict) return;
    setResource(conflict);
    form.setFieldsValue(resolveThemeSettings(inherited, undefined, conflict.overrides));
    setDirtyKeys(new Set());
    setConflict(undefined);
  };

  const reviewConflict = () => {
    // Keep the touched draft, but make the newly received canonical revision
    // the base for the user's next explicit save.
    setConflict(undefined);
  };

  if (loading) {
    return (
      <>
        {messageContextHolder}
        <Skeleton active paragraph={{ rows: 8 }} />
      </>
    );
  }

  if (error && !hasLoadedCanonicalResource) {
    return (
      <>
        {messageContextHolder}
        <Form<ThemeSettings> form={form} style={{ display: 'none' }} />
        <Alert
          action={
            permissionDenied ? undefined : (
              <Button htmlType="button" onClick={() => void load()} size="small">
                {intl.formatMessage({ id: 'pages.theme.settings.retry', defaultMessage: 'Retry' })}
              </Button>
            )
          }
          message={
            permissionDenied
              ? intl.formatMessage({
                  id: 'pages.theme.settings.permissionDenied',
                  defaultMessage: 'You do not have permission to view the application theme.',
                })
              : intl.formatMessage({
                  id: 'pages.theme.settings.error',
                  defaultMessage: 'Unable to update theme settings',
                })
          }
          showIcon
          type="error"
        />
      </>
    );
  }

  return (
    <>
      {messageContextHolder}
      <Form<ThemeSettings>
        disabled={!canMutate || mutating}
        form={form}
        layout="vertical"
        onFinish={submit}
        onValuesChange={(changedValues, allValues) => {
          if (mutating) return;
          setDirtyKeys((previous) => {
            const next = new Set(previous);
            (Object.keys(changedValues) as ThemeSettingKey[]).forEach((key) => {
              const normalized = normalizeThemeOverrides({ [key]: allValues[key] });
              if (normalized[key] === effective[key]) {
                next.delete(key);
              } else {
                next.add(key);
              }
            });
            return next;
          });
        }}
      >
        {!hasMutationPermission && scope === 'application' && (
          <Alert
            message={intl.formatMessage({
              id: 'pages.theme.settings.readOnly',
              defaultMessage:
                'You can view the application theme but need configuration write permission to change it.',
            })}
            showIcon
            style={{ marginBottom: 16 }}
            type="info"
          />
        )}

        {identityMismatch && (
          <Alert
            message={intl.formatMessage({
              id: 'pages.theme.settings.identityChanged',
              defaultMessage:
                'The signed-in identity changed in another tab. Reload or sign in again before editing theme settings.',
            })}
            showIcon
            style={{ marginBottom: 16 }}
            type="warning"
          />
        )}

        {error && (
          <Alert
            action={
              <Button htmlType="button" onClick={() => void load()} size="small">
                {intl.formatMessage({ id: 'pages.theme.settings.retry', defaultMessage: 'Retry' })}
              </Button>
            }
            message={intl.formatMessage({
              id: 'pages.theme.settings.error',
              defaultMessage: 'Unable to update theme settings',
            })}
            showIcon
            style={{ marginBottom: 16 }}
            type="error"
          />
        )}

        {degraded && !error && (
          <Alert
            message={intl.formatMessage({
              id: 'pages.theme.settings.degraded',
              defaultMessage:
                'The latest theme could not be verified. A valid recent snapshot is being used.',
            })}
            showIcon
            style={{ marginBottom: 16 }}
            type="warning"
          />
        )}

        {conflict && (
          <Alert
            action={
              <Space wrap>
                <Button disabled={mutating} htmlType="button" onClick={reloadConflict} size="small">
                  {intl.formatMessage({
                    id: 'pages.theme.settings.conflict.reload',
                    defaultMessage: 'Reload latest',
                  })}
                </Button>
                <Button
                  disabled={mutating}
                  htmlType="button"
                  onClick={reviewConflict}
                  size="small"
                  type="primary"
                >
                  {intl.formatMessage({
                    id: 'pages.theme.settings.conflict.review',
                    defaultMessage: 'Review and retry',
                  })}
                </Button>
              </Space>
            }
            description={intl.formatMessage({
              id: 'pages.theme.settings.conflict.description',
              defaultMessage:
                'Your draft was preserved. Reload the latest settings or review your changes before saving again.',
            })}
            message={intl.formatMessage({
              id: 'pages.theme.settings.conflict',
              defaultMessage: 'Theme settings changed in another tab',
            })}
            showIcon
            style={{ marginBottom: 16 }}
            type="warning"
          />
        )}

        <Form.Item label={label('navTheme', fieldIntl(intl, 'navTheme'))} name="navTheme">
          <Select
            disabled={!canMutate || mutating}
            options={[
              {
                label: intl.formatMessage({
                  id: 'pages.theme.settings.option.navTheme.light',
                  defaultMessage: 'Light',
                }),
                value: 'light',
              },
              {
                label: intl.formatMessage({
                  id: 'pages.theme.settings.option.navTheme.dark',
                  defaultMessage: 'Dark',
                }),
                value: 'realDark',
              },
            ]}
          />
        </Form.Item>
        <Form.Item
          getValueFromEvent={(color) =>
            typeof color === 'string' ? color : color?.toHexString?.()
          }
          label={label('colorPrimary', fieldIntl(intl, 'primaryColor'))}
          name="colorPrimary"
        >
          <ColorPicker disabled={!canMutate || mutating} format="hex" showText />
        </Form.Item>
        <Form.Item label={label('layout', fieldIntl(intl, 'layout'))} name="layout">
          <Select
            disabled={!canMutate || mutating}
            options={[
              {
                label: intl.formatMessage({
                  id: 'pages.theme.settings.option.layout.side',
                  defaultMessage: 'Side',
                }),
                value: 'side',
              },
              {
                label: intl.formatMessage({
                  id: 'pages.theme.settings.option.layout.top',
                  defaultMessage: 'Top',
                }),
                value: 'top',
              },
              {
                label: intl.formatMessage({
                  id: 'pages.theme.settings.option.layout.mix',
                  defaultMessage: 'Mix',
                }),
                value: 'mix',
              },
            ]}
          />
        </Form.Item>
        <Form.Item
          label={label('contentWidth', fieldIntl(intl, 'contentWidth'))}
          name="contentWidth"
        >
          <Select
            disabled={!canMutate || mutating}
            options={[
              {
                label: intl.formatMessage({
                  id: 'pages.theme.settings.option.contentWidth.fluid',
                  defaultMessage: 'Fluid',
                }),
                value: 'Fluid',
              },
              {
                label: intl.formatMessage({
                  id: 'pages.theme.settings.option.contentWidth.fixed',
                  defaultMessage: 'Fixed',
                }),
                value: 'Fixed',
              },
            ]}
          />
        </Form.Item>
        <Form.Item
          label={label('fixedHeader', fieldIntl(intl, 'fixedHeader'))}
          name="fixedHeader"
          valuePropName="checked"
        >
          <Switch disabled={!canMutate || mutating} />
        </Form.Item>
        <Form.Item
          label={label('fixSiderbar', fieldIntl(intl, 'fixSiderbar'))}
          name="fixSiderbar"
          valuePropName="checked"
        >
          <Switch disabled={!canMutate || mutating} />
        </Form.Item>
        <Form.Item
          label={label('colorWeak', fieldIntl(intl, 'colorWeak'))}
          name="colorWeak"
          valuePropName="checked"
        >
          <Switch disabled={!canMutate || mutating} />
        </Form.Item>

        <Space wrap>
          <Button
            disabled={!canMutate || mutating || !dirty || Boolean(conflict)}
            htmlType="submit"
            loading={saving}
            type="primary"
          >
            {intl.formatMessage({ id: 'pages.theme.settings.save', defaultMessage: 'Save theme' })}
          </Button>
          <Popconfirm
            cancelText={intl.formatMessage({ id: 'pages.title.cancel', defaultMessage: 'Cancel' })}
            disabled={
              !canMutate || mutating || Boolean(conflict) || Object.keys(overrides).length === 0
            }
            okText={intl.formatMessage({ id: 'pages.title.ok', defaultMessage: 'OK' })}
            onConfirm={() => void resetAll()}
            title={intl.formatMessage({
              id: 'pages.theme.settings.resetAll.confirm',
              defaultMessage: 'Restore every theme setting to its inherited value?',
            })}
          >
            <Button
              disabled={
                !canMutate || mutating || Boolean(conflict) || Object.keys(overrides).length === 0
              }
              htmlType="button"
              loading={resettingAll}
            >
              {intl.formatMessage({
                id: 'pages.theme.settings.resetAll',
                defaultMessage: 'Restore all inherited settings',
              })}
            </Button>
          </Popconfirm>
        </Space>
      </Form>
    </>
  );
};

export default Theme;
