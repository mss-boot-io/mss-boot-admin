import {
  CheckOutlined,
  CloudSyncOutlined,
  DeleteOutlined,
  ReloadOutlined,
  SaveOutlined,
  UndoOutlined,
} from '@ant-design/icons';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useIntl, useModel } from '@umijs/max';
import {
  Alert,
  App,
  Button,
  Card,
  ColorPicker,
  Form,
  Popconfirm,
  Select,
  Space,
  Switch,
  Tag,
  Typography,
} from 'antd';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { getRequestStatus } from '@/shared/api/errors';
import { hasPermission } from '@/shared/auth/access';
import { PageError, PageForbidden, PageLoading } from '@/shared/design-system/PageState';
import { queryKeys } from '@/shared/query/client';
import {
  loadThemeResource,
  patchThemeResource,
  resetThemeResource,
  ThemeRevisionConflictError,
} from '@/shared/theme/api';
import {
  areThemeResourcesEqual,
  CODE_THEME_DEFAULTS,
  hasThemeOverride,
  normalizeThemeColor,
  resolveTheme,
  THEME_SETTING_KEYS,
  type ThemePatch,
  type ThemeScope,
  type ThemeScopeResource,
  type ThemeSettingKey,
  type ThemeSettings,
  type ThemeSource,
} from '@/shared/theme/contract';
import { getThemeRuntimeSnapshot, subscribeThemeRuntime } from '@/shared/theme/runtime';
import { writeThemeSnapshot } from '@/shared/theme/snapshot';
import { publishThemeScopeResource } from '@/shared/theme/sync';
import { applyCanonicalThemeResource, mergeAppliedThemeIntoInitialState } from './apply';

type ThemeFormValues = ThemeSettings;

interface ThemeSettingsEditorProps {
  scope: ThemeScope;
}

const fieldMessageIDs: Record<ThemeSettingKey, string> = {
  navTheme: 'theme.field.navTheme',
  colorPrimary: 'theme.field.colorPrimary',
  layout: 'theme.field.layout',
  contentWidth: 'theme.field.contentWidth',
  fixedHeader: 'theme.field.fixedHeader',
  fixSiderbar: 'theme.field.fixSiderbar',
  colorWeak: 'theme.field.colorWeak',
};

function resourceValues(
  scope: ThemeScope,
  resource: ThemeScopeResource,
  application?: ThemeScopeResource,
): ThemeSettings {
  return scope === 'application'
    ? resolveTheme(resource).settings
    : resolveTheme(application, resource).settings;
}

function sourceColor(source: ThemeSource) {
  if (source === 'user') return 'purple';
  if (source === 'application') return 'blue';
  return 'default';
}

export default function ThemeSettingsEditor({ scope }: ThemeSettingsEditorProps) {
  const intl = useIntl();
  const { message } = App.useApp();
  const { initialState, setInitialState } = useModel('@@initialState');
  const client = useQueryClient();
  const [form] = Form.useForm<ThemeFormValues>();
  const runtime = useThemeRuntime();
  const owner = scope === 'user' ? (initialState?.currentUser?.id ?? '') : '';
  const queryKey = queryKeys.theme(scope, owner);
  const [base, setBase] = useState<ThemeScopeResource>();
  const [dirtyKeys, setDirtyKeys] = useState<Set<ThemeSettingKey>>(() => new Set());
  const dirtyKeysRef = useRef(dirtyKeys);
  const [conflict, setConflict] = useState<ThemeScopeResource>();

  const canRead =
    scope === 'user'
      ? Boolean(initialState?.currentUser)
      : hasPermission(initialState?.currentUser, '/app-config');
  const canWrite =
    scope === 'user'
      ? Boolean(initialState?.currentUser)
      : hasPermission(initialState?.currentUser, '/app-config/control');

  useEffect(() => {
    dirtyKeysRef.current = dirtyKeys;
  }, [dirtyKeys]);

  const resourceQuery = useQuery({
    queryKey,
    queryFn: () => loadThemeResource(scope),
    enabled: canRead && (scope === 'application' || owner.length > 0),
    staleTime: 0,
    retry: false,
  });

  const applyResource = useCallback(
    (resource: ThemeScopeResource) => {
      const applied = applyCanonicalThemeResource(client, resource, owner);
      void setInitialState((previous) => mergeAppliedThemeIntoInitialState(previous, applied));
      return applied.resource;
    },
    [client, owner, setInitialState],
  );

  useEffect(() => {
    if (!resourceQuery.data) return;
    const accepted = applyResource(resourceQuery.data);
    setBase((current) => {
      if (!current) {
        form.setFieldsValue(resourceValues(scope, accepted, getThemeRuntimeSnapshot().application));
        return accepted;
      }
      if (areThemeResourcesEqual(current, accepted)) return current;
      if (dirtyKeysRef.current.size > 0) {
        setConflict(accepted);
        return current;
      }
      form.setFieldsValue(resourceValues(scope, accepted, getThemeRuntimeSnapshot().application));
      setConflict(undefined);
      return accepted;
    });
  }, [applyResource, form, resourceQuery.data, scope]);

  const acceptMutation = useCallback(
    (resource: ThemeScopeResource, preservedKeys: ReadonlySet<ThemeSettingKey> = new Set()) => {
      const draft = form.getFieldsValue();
      const accepted = applyResource(resource);
      const authSessionId = scope === 'user' ? initialState?.authSessionId : undefined;
      publishThemeScopeResource(accepted, authSessionId);
      void writeThemeSnapshot(accepted, authSessionId);
      setBase(accepted);
      setConflict(undefined);
      setDirtyKeys(new Set(preservedKeys));
      const canonicalValues = resourceValues(
        scope,
        accepted,
        getThemeRuntimeSnapshot().application,
      );
      for (const key of preservedKeys) Object.assign(canonicalValues, { [key]: draft[key] });
      form.setFieldsValue(canonicalValues);
      return accepted;
    },
    [applyResource, form, initialState?.authSessionId, scope],
  );

  const exposeConflict = useCallback(
    async (error: unknown) => {
      if (!(error instanceof ThemeRevisionConflictError)) return false;
      let latest = error.current;
      if (!latest) latest = await loadThemeResource(scope);
      const accepted = applyResource(latest);
      setConflict(accepted);
      return true;
    },
    [applyResource, scope],
  );

  const saveMutation = useMutation({
    mutationFn: async () => {
      if (!base) throw new Error('Theme resource has not loaded');
      const values = await form.validateFields();
      const patch = Object.fromEntries(
        [...dirtyKeys].map((key) => [key, values[key]]),
      ) as ThemePatch;
      return patchThemeResource(scope, patch, base);
    },
    onSuccess: (resource) => {
      acceptMutation(resource);
      void message.success(intl.formatMessage({ id: 'theme.save.success' }));
    },
    onError: async (error) => {
      try {
        if (await exposeConflict(error)) return;
      } catch (latestError) {
        void message.error(String(latestError));
        return;
      }
      void message.error(intl.formatMessage({ id: 'theme.save.failure' }));
    },
  });

  const resetMutation = useMutation({
    mutationFn: async (keys?: readonly ThemeSettingKey[]) => {
      if (!base) throw new Error('Theme resource has not loaded');
      return resetThemeResource(scope, base, keys);
    },
    onSuccess: (resource, keys) => {
      const preservedKeys = keys
        ? new Set([...dirtyKeys].filter((key) => !keys.includes(key)))
        : new Set<ThemeSettingKey>();
      acceptMutation(resource, preservedKeys);
      void message.success(
        intl.formatMessage({ id: keys ? 'theme.inherit.success' : 'theme.reset.success' }),
      );
    },
    onError: async (error) => {
      try {
        if (await exposeConflict(error)) return;
      } catch (latestError) {
        void message.error(String(latestError));
        return;
      }
      void message.error(intl.formatMessage({ id: 'theme.save.failure' }));
    },
  });

  const canonical = conflict ?? base ?? runtime[scope];
  const resolved = useMemo(
    () =>
      scope === 'application'
        ? resolveTheme(canonical)
        : resolveTheme(runtime.application, canonical),
    [canonical, runtime.application, scope],
  );
  const inherited = useMemo(
    () =>
      scope === 'application' ? CODE_THEME_DEFAULTS : resolveTheme(runtime.application).settings,
    [runtime.application, scope],
  );
  const mutating = saveMutation.isPending || resetMutation.isPending;
  const degraded = runtime.degradedScopes.includes(scope);

  if (!canRead) {
    return <PageForbidden message={intl.formatMessage({ id: 'states.forbidden' })} />;
  }
  if (resourceQuery.isPending && !base) return <PageLoading rows={7} />;
  if (resourceQuery.isError && !base) {
    if (getRequestStatus(resourceQuery.error) === 403) {
      return <PageForbidden message={intl.formatMessage({ id: 'states.forbidden' })} />;
    }
    return (
      <PageError
        title={intl.formatMessage({ id: 'states.loadError' })}
        message={intl.formatMessage({ id: 'theme.load.failure' })}
        retryLabel={intl.formatMessage({ id: 'actions.retry' })}
        onRetry={() => void resourceQuery.refetch()}
      />
    );
  }
  if (!base || !canonical) return <PageLoading rows={7} />;

  const markDirty = (changed: Partial<ThemeFormValues>) => {
    setDirtyKeys((previous) => {
      const next = new Set(previous);
      for (const key of Object.keys(changed) as ThemeSettingKey[]) next.add(key);
      return next;
    });
  };

  const resolveConflict = (keepDraft: boolean) => {
    if (!conflict) return;
    setBase(conflict);
    setConflict(undefined);
    if (!keepDraft) {
      setDirtyKeys(new Set());
      form.setFieldsValue(resourceValues(scope, conflict, getThemeRuntimeSnapshot().application));
    }
  };

  const sourceLabel = (source: ThemeSource) => intl.formatMessage({ id: `theme.source.${source}` });

  const renderControl = (key: ThemeSettingKey) => {
    if (key === 'navTheme') {
      return (
        <Select
          options={[
            { value: 'light', label: intl.formatMessage({ id: 'theme.value.light' }) },
            { value: 'realDark', label: intl.formatMessage({ id: 'theme.value.realDark' }) },
          ]}
        />
      );
    }
    if (key === 'layout') {
      return (
        <Select
          options={['side', 'top', 'mix'].map((value) => ({
            value,
            label: intl.formatMessage({ id: `theme.value.layout.${value}` }),
          }))}
        />
      );
    }
    if (key === 'contentWidth') {
      return (
        <Select
          options={['Fluid', 'Fixed'].map((value) => ({
            value,
            label: intl.formatMessage({ id: `theme.value.width.${value}` }),
          }))}
        />
      );
    }
    if (key === 'colorPrimary') {
      return (
        <ColorPicker
          showText
          format="hex"
          getPopupContainer={(trigger) => trigger.parentElement ?? document.body}
        />
      );
    }
    return <Switch />;
  };

  return (
    <Space orientation="vertical" size="large" className="w-full">
      {degraded ? (
        <Alert
          showIcon
          type="warning"
          title={intl.formatMessage({ id: 'theme.degraded.title' })}
          description={intl.formatMessage({ id: 'theme.degraded.description' })}
          action={
            <Button icon={<ReloadOutlined />} onClick={() => void resourceQuery.refetch()}>
              {intl.formatMessage({ id: 'actions.retry' })}
            </Button>
          }
        />
      ) : null}
      {!canWrite ? (
        <Alert showIcon type="info" title={intl.formatMessage({ id: 'theme.readOnly' })} />
      ) : null}
      {conflict ? (
        <Alert
          showIcon
          type="warning"
          title={intl.formatMessage({ id: 'theme.conflict.title' })}
          description={intl.formatMessage(
            { id: 'theme.conflict.description' },
            { revision: conflict.revision },
          )}
          action={
            <Space wrap>
              <Button onClick={() => resolveConflict(false)}>
                {intl.formatMessage({ id: 'theme.conflict.discard' })}
              </Button>
              <Button type="primary" onClick={() => resolveConflict(true)}>
                {intl.formatMessage({ id: 'theme.conflict.keep' })}
              </Button>
            </Space>
          }
        />
      ) : null}
      <Card
        title={intl.formatMessage({
          id: scope === 'application' ? 'theme.scope.application' : 'theme.scope.user',
        })}
        extra={
          <Space wrap>
            <Typography.Text type="secondary">
              {intl.formatMessage({ id: 'theme.revision' }, { revision: canonical.revision })}
            </Typography.Text>
            <Button
              icon={<SaveOutlined />}
              type="primary"
              disabled={!canWrite || dirtyKeys.size === 0 || Boolean(conflict)}
              loading={saveMutation.isPending}
              onClick={() => saveMutation.mutate()}
            >
              {intl.formatMessage({ id: 'actions.save' })}
            </Button>
            <Popconfirm
              title={intl.formatMessage({ id: 'theme.reset.confirm' })}
              onConfirm={() => resetMutation.mutate(undefined)}
              okButtonProps={{ danger: true }}
            >
              <Button
                danger
                icon={<DeleteOutlined />}
                disabled={!canWrite || Object.keys(canonical.overrides).length === 0}
                loading={resetMutation.isPending}
              >
                {intl.formatMessage({ id: 'theme.reset.all' })}
              </Button>
            </Popconfirm>
          </Space>
        }
      >
        <Alert
          className="mb-5"
          showIcon
          type="info"
          icon={<CloudSyncOutlined />}
          title={intl.formatMessage({ id: 'theme.precedence.title' })}
          description={intl.formatMessage({ id: 'theme.precedence.description' })}
        />
        <Form<ThemeFormValues>
          form={form}
          layout="vertical"
          disabled={mutating || !canWrite}
          onValuesChange={markDirty}
        >
          <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
            {THEME_SETTING_KEYS.map((key) => {
              const explicit = hasThemeOverride(canonical.overrides, key);
              const dirty = dirtyKeys.has(key);
              const source = resolved.sources[key];
              return (
                <Card key={key} size="small" className="min-w-0" styles={{ body: { padding: 16 } }}>
                  <div className="mb-3 flex flex-wrap items-start justify-between gap-2">
                    <Space wrap>
                      <Typography.Text strong>
                        {intl.formatMessage({ id: fieldMessageIDs[key] })}
                      </Typography.Text>
                      <Tag color={sourceColor(source)}>{sourceLabel(source)}</Tag>
                      {dirty ? (
                        <Tag icon={<CheckOutlined />} color="gold">
                          {intl.formatMessage({ id: 'theme.source.draft' })}
                        </Tag>
                      ) : null}
                    </Space>
                    <Button
                      type="link"
                      size="small"
                      icon={<UndoOutlined />}
                      disabled={!canWrite || (!explicit && !dirty) || Boolean(conflict)}
                      loading={resetMutation.isPending && resetMutation.variables?.[0] === key}
                      onClick={() => resetMutation.mutate([key])}
                    >
                      {intl.formatMessage({ id: 'theme.inherit' })}
                    </Button>
                  </div>
                  <Form.Item
                    name={key}
                    className="mb-2"
                    valuePropName={
                      key === 'fixedHeader' || key === 'fixSiderbar' || key === 'colorWeak'
                        ? 'checked'
                        : 'value'
                    }
                    getValueFromEvent={
                      key === 'colorPrimary'
                        ? (_color, css: string) => normalizeThemeColor(css) ?? css
                        : undefined
                    }
                    rules={
                      key === 'colorPrimary'
                        ? [
                            {
                              validator: async (_, value) => {
                                if (!normalizeThemeColor(value)) {
                                  throw new Error(
                                    intl.formatMessage({ id: 'theme.color.invalid' }),
                                  );
                                }
                              },
                            },
                          ]
                        : undefined
                    }
                  >
                    {renderControl(key)}
                  </Form.Item>
                  <Typography.Text type="secondary" className="text-xs">
                    {intl.formatMessage(
                      { id: 'theme.inheritedValue' },
                      { value: String(inherited[key]) },
                    )}
                  </Typography.Text>
                </Card>
              );
            })}
          </div>
        </Form>
      </Card>
    </Space>
  );
}

function useThemeRuntime() {
  const [runtime, setRuntime] = useState(getThemeRuntimeSnapshot);
  useEffect(() => subscribeThemeRuntime(() => setRuntime(getThemeRuntimeSnapshot())), []);
  return runtime;
}
