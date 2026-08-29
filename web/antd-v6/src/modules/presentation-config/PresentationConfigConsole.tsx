import { getRequestErrorMessage, getRequestStatus } from '@mss-admin-core/shared/api/errors';
import {
  PageEmpty,
  PageError,
  PageForbidden,
  PageLoading,
} from '@mss-admin-core/shared/design-system/PageState';
import { queryKeys } from '@mss-admin-core/shared/query/client';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import {
  Alert,
  App,
  Button,
  Card,
  Col,
  Descriptions,
  Input,
  Popconfirm,
  Row,
  Segmented,
  Select,
  Space,
  Table,
  type TableColumnsType,
  Tag,
  Typography,
} from 'antd';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { newPresentationIdempotencyKey, presentationAPI } from './api';
import {
  buildInitialPresentationDocument,
  formatPresentationDocument,
  type PresentationCapability,
  type PresentationProfile,
  type PresentationProfileIdentity,
  type PresentationProfileSummary,
  type PresentationRevisionSummary,
  type PresentationScope,
  type PresentationValidationResult,
  parsePresentationDocumentText,
  presentationConflictVersion,
} from './contract';
import { usePresentationIntl } from './messages';
import PresentationVisualEditor from './PresentationVisualEditor';
import { formatPresentationDraftAST, parsePresentationDraftAST } from './presentationDraftAst';
import { buildPresentationPreview } from './preview';
import {
  usePresentationCapabilities,
  usePresentationProfile,
  usePresentationProfiles,
  usePresentationPublishedRevision,
  usePresentationRevisions,
} from './query';

interface PresentationConfigConsoleProps {
  canDraft: boolean;
  canPublish: boolean;
  canRollback: boolean;
}

function capabilityLabel(capability: PresentationCapability, locale: string): string {
  const title = capability.defaultPresentation.title;
  if (title && typeof title === 'object' && !Array.isArray(title)) {
    const localized = title as Record<string, unknown>;
    const preferred = locale.startsWith('zh') ? localized['zh-CN'] : localized['en-US'];
    if (typeof preferred === 'string' && preferred.trim()) return preferred;
  }
  return capability.pageKey;
}

function profileLabel(profile: PresentationProfileSummary): string {
  const subject = profile.subjectID ? ` · ${profile.subjectID}` : '';
  return `${profile.pageKey} · ${profile.scope}${subject}`;
}

function profileStateTag(
  profile: PresentationProfileSummary,
  validLabel: string,
  invalidLabel: string,
) {
  if (profile.state === 'published') return <Tag color="green">published</Tag>;
  return (
    <Space size={4}>
      <Tag color="blue">draft</Tag>
      {profile.draftValid === true ? (
        <Tag color="success">{validLabel}</Tag>
      ) : (
        <Tag color="error">{invalidLabel}</Tag>
      )}
    </Space>
  );
}

export default function PresentationConfigConsole({
  canDraft,
  canPublish,
  canRollback,
}: PresentationConfigConsoleProps) {
  const intl = usePresentationIntl();
  const { message } = App.useApp();
  const queryClient = useQueryClient();
  const capabilities = usePresentationCapabilities();
  const capabilityItems = capabilities.data?.items ?? [];
  const [profilePage, setProfilePage] = useState(1);
  const [historyPage, setHistoryPage] = useState(1);
  const profiles = usePresentationProfiles(profilePage, 10);
  const [selectedID, setSelectedID] = useState<string>();
  const [creating, setCreating] = useState(false);
  const [pageKey, setPageKey] = useState('');
  const [scope, setScope] = useState<PresentationScope>('application');
  const [subjectID, setSubjectID] = useState('');
  const [editorText, setEditorText] = useState('');
  const [editorMode, setEditorMode] = useState<'raw' | 'visual'>('raw');
  const [dirty, setDirty] = useState(false);
  const [localError, setLocalError] = useState<string>();
  const [validation, setValidation] = useState<PresentationValidationResult>();
  const [conflict, setConflict] = useState<number>();
  const loadedSourceKey = useRef('');

  const profile = usePresentationProfile(selectedID);
  const revisions = usePresentationRevisions(selectedID, historyPage, 10);
  const publishedDocument = usePresentationPublishedRevision(
    profile.data?.draft ? undefined : profile.data?.id,
    profile.data?.draft ? undefined : profile.data?.publishedRevision,
  );
  const selectedCapability = capabilityItems.find(
    (item) => item.pageKey === (profile.data?.pageKey ?? pageKey),
  );
  const preview = useMemo(() => {
    if (
      !selectedCapability ||
      !validation?.structurallyValid ||
      !validation.semanticallyValid ||
      !validation.canonicalDocument
    ) {
      return undefined;
    }
    try {
      return buildPresentationPreview(
        selectedCapability,
        validation.canonicalDocument,
        intl.locale.startsWith('zh') ? 'zh-CN' : 'en-US',
      );
    } catch {
      return undefined;
    }
  }, [intl.locale, selectedCapability, validation]);
  const visualDocument = useMemo(() => {
    if (!editorText) return undefined;
    try {
      return parsePresentationDraftAST(editorText);
    } catch {
      return undefined;
    }
  }, [editorText]);

  const identity = useMemo<PresentationProfileIdentity>(
    () => ({
      pageKey,
      scope,
      subjectID: scope === 'application' ? '' : subjectID.trim(),
    }),
    [pageKey, scope, subjectID],
  );

  const resetTemplate = useCallback(
    (capability: PresentationCapability, nextIdentity: PresentationProfileIdentity) => {
      setEditorText(
        formatPresentationDocument(buildInitialPresentationDocument(capability, nextIdentity)),
      );
      setDirty(false);
      setLocalError(undefined);
      setValidation(undefined);
      setConflict(undefined);
    },
    [],
  );

  useEffect(() => {
    const firstCapability = capabilityItems[0];
    if (!firstCapability || pageKey || !profiles.data || profiles.isFetching) return;
    setPageKey(firstCapability.pageKey);
    setCreating(profiles.data.items.length === 0);
  }, [capabilityItems, pageKey, profiles.data, profiles.isFetching]);

  useEffect(() => {
    if (selectedID || creating || !profiles.data?.items[0]) return;
    setSelectedID(profiles.data.items[0].id);
  }, [creating, profiles.data?.items, selectedID]);

  useEffect(() => {
    if (!creating || !selectedCapability || editorText) return;
    resetTemplate(selectedCapability, identity);
  }, [creating, editorText, identity, resetTemplate, selectedCapability]);

  const sourceDocument = profile.data?.draft?.document ?? publishedDocument.data?.document;
  const sourceKey = profile.data
    ? `${profile.data.id}:${profile.data.version}:${profile.data.draft?.digest ?? profile.data.publishedRevision}`
    : '';
  useEffect(() => {
    if (
      creating ||
      !sourceDocument ||
      !sourceKey ||
      sourceKey === loadedSourceKey.current ||
      conflict
    ) {
      return;
    }
    setEditorText(formatPresentationDocument(sourceDocument));
    setDirty(false);
    setLocalError(undefined);
    setValidation(undefined);
    loadedSourceKey.current = sourceKey;
  }, [conflict, creating, sourceDocument, sourceKey]);

  const refreshProfileCollections = async (current: PresentationProfile) => {
    queryClient.setQueryData(queryKeys.presentationProfile(current.id), current);
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: queryKeys.presentationProfiles }),
      queryClient.invalidateQueries({ queryKey: queryKeys.presentationRevisions(current.id) }),
    ]);
  };

  const validate = useMutation({
    mutationFn: async () => presentationAPI.validate(parsePresentationDocumentText(editorText)),
    onSuccess: (result) => {
      setValidation(result);
      setLocalError(undefined);
    },
    onError: (error) => {
      setValidation(undefined);
      setLocalError(getRequestErrorMessage(error));
    },
  });

  const save = useMutation({
    mutationFn: async () => {
      const document = parsePresentationDocumentText(editorText);
      if (creating) return presentationAPI.createDraft(identity, document);
      if (!profile.data) throw new Error('Presentation profile revision is unavailable');
      return presentationAPI.replaceDraft(profile.data, document);
    },
    onSuccess: async (current) => {
      setSelectedID(current.id);
      setCreating(false);
      setConflict(undefined);
      setDirty(false);
      setLocalError(undefined);
      setValidation(undefined);
      loadedSourceKey.current = `${current.id}:${current.version}:${current.draft?.digest ?? current.publishedRevision}`;
      if (current.draft) setEditorText(formatPresentationDocument(current.draft.document));
      await refreshProfileCollections(current);
      void message.success(intl.formatMessage({ id: 'presentation.save.success' }));
    },
    onError: (error) => {
      setConflict(presentationConflictVersion(error));
      setLocalError(getRequestErrorMessage(error));
    },
  });

  const publish = useMutation({
    mutationFn: async () => {
      if (!profile.data) throw new Error('Presentation profile revision is unavailable');
      return presentationAPI.publish(profile.data, newPresentationIdempotencyKey('publish'));
    },
    onSuccess: async (result) => {
      setEditorText(formatPresentationDocument(result.revision.document));
      setDirty(false);
      setConflict(undefined);
      setLocalError(undefined);
      queryClient.setQueryData(
        queryKeys.presentationRevision(result.profile.id, result.revision.revision),
        result.revision,
      );
      loadedSourceKey.current = `${result.profile.id}:${result.profile.version}:${result.profile.publishedRevision}`;
      await refreshProfileCollections(result.profile);
      void message.success(intl.formatMessage({ id: 'presentation.publish.success' }));
    },
    onError: (error) => {
      setConflict(presentationConflictVersion(error));
      setLocalError(getRequestErrorMessage(error));
    },
  });

  const rollback = useMutation({
    mutationFn: async (revision: number) => {
      if (!profile.data) throw new Error('Presentation profile revision is unavailable');
      return presentationAPI.rollback(
        profile.data,
        revision,
        newPresentationIdempotencyKey('rollback'),
      );
    },
    onSuccess: async (result) => {
      setEditorText(formatPresentationDocument(result.revision.document));
      setDirty(false);
      setConflict(undefined);
      setLocalError(undefined);
      queryClient.setQueryData(
        queryKeys.presentationRevision(result.profile.id, result.revision.revision),
        result.revision,
      );
      loadedSourceKey.current = `${result.profile.id}:${result.profile.version}:${result.profile.publishedRevision}`;
      await refreshProfileCollections(result.profile);
      void message.success(intl.formatMessage({ id: 'presentation.rollback.success' }));
    },
    onError: (error) => {
      setConflict(presentationConflictVersion(error));
      setLocalError(getRequestErrorMessage(error));
    },
  });

  const statuses = [
    capabilities.error,
    profiles.error,
    profile.error,
    revisions.error,
    publishedDocument.error,
    validate.error,
    save.error,
    publish.error,
    rollback.error,
  ].map(getRequestStatus);
  if (statuses.includes(403)) {
    return <PageForbidden message={intl.formatMessage({ id: 'presentation.forbidden' })} />;
  }
  if ((capabilities.isPending || profiles.isPending) && !capabilities.data && !profiles.data) {
    return <PageLoading rows={10} />;
  }
  if (capabilities.isError || profiles.isError) {
    const error = capabilities.error ?? profiles.error;
    return (
      <PageError
        message={getRequestErrorMessage(error)}
        onRetry={() => {
          void capabilities.refetch();
          void profiles.refetch();
        }}
      />
    );
  }
  const recoveryNotice = capabilities.data?.recoveryMode ? (
    <Alert title={intl.formatMessage({ id: 'presentation.recovery.title' })} type="warning" />
  ) : null;
  if (!capabilityItems.length && !profiles.data?.items.length) {
    return (
      recoveryNotice ?? (
        <PageEmpty description={intl.formatMessage({ id: 'presentation.capabilities.empty' })} />
      )
    );
  }

  const beginCreate = () => {
    const capability = capabilityItems[0];
    if (!capability) return;
    const nextIdentity: PresentationProfileIdentity = {
      pageKey: capability.pageKey,
      scope: 'application',
      subjectID: '',
    };
    setSelectedID(undefined);
    setCreating(true);
    setPageKey(capability.pageKey);
    setScope('application');
    setSubjectID('');
    loadedSourceKey.current = '';
    resetTemplate(capability, nextIdentity);
  };

  const selectProfile = (id: string) => {
    setCreating(false);
    setSelectedID(id);
    setConflict(undefined);
    setValidation(undefined);
    setLocalError(undefined);
    setHistoryPage(1);
    loadedSourceKey.current = '';
  };

  const discardAndReload = async () => {
    setConflict(undefined);
    setDirty(false);
    setValidation(undefined);
    setLocalError(undefined);
    loadedSourceKey.current = '';
    await Promise.all([profile.refetch(), profiles.refetch()]);
  };

  const openRawIssue = (path: string) => {
    setEditorMode('raw');
    globalThis.requestAnimationFrame(() => {
      const editor = globalThis.document.getElementById(
        'presentation-profile-json',
      ) as HTMLTextAreaElement | null;
      if (!editor) return;
      editor.focus();
      const property = path
        .split('.')
        .at(-1)
        ?.replace(/\[\d+\]/g, '');
      const offset = property ? editor.value.indexOf(`"${property}"`) : -1;
      if (offset >= 0) editor.setSelectionRange(offset, offset + (property?.length ?? 0) + 2);
    });
  };

  const profileColumns: TableColumnsType<PresentationProfileSummary> = [
    {
      title: intl.formatMessage({ id: 'presentation.profile' }),
      key: 'identity',
      render: (_, item) => (
        <Button type="link" className="px-0" onClick={() => selectProfile(item.id)}>
          {profileLabel(item)}
        </Button>
      ),
    },
    {
      title: intl.formatMessage({ id: 'presentation.state' }),
      key: 'state',
      width: 190,
      render: (_, item) =>
        profileStateTag(
          item,
          intl.formatMessage({ id: 'presentation.valid' }),
          intl.formatMessage({ id: 'presentation.invalid' }),
        ),
    },
    {
      title: intl.formatMessage({ id: 'presentation.version' }),
      dataIndex: 'version',
      width: 90,
    },
  ];

  const historyColumns: TableColumnsType<PresentationRevisionSummary> = [
    {
      title: intl.formatMessage({ id: 'presentation.revision' }),
      dataIndex: 'revision',
      width: 90,
    },
    {
      title: intl.formatMessage({ id: 'presentation.transition' }),
      dataIndex: 'transition',
      width: 120,
      render: (value: PresentationRevisionSummary['transition']) => (
        <Tag color={value === 'publish' ? 'blue' : 'gold'}>{value}</Tag>
      ),
    },
    {
      title: intl.formatMessage({ id: 'presentation.actor' }),
      dataIndex: 'actorID',
      ellipsis: true,
    },
    {
      title: intl.formatMessage({ id: 'presentation.createdAt' }),
      dataIndex: 'createdAt',
      render: (value: string) =>
        new Intl.DateTimeFormat(intl.locale || 'zh-CN', {
          dateStyle: 'short',
          timeStyle: 'medium',
        }).format(new Date(value)),
    },
    {
      title: intl.formatMessage({ id: 'presentation.actions' }),
      key: 'actions',
      width: 140,
      render: (_, item) =>
        canRollback ? (
          <Popconfirm
            description={intl.formatMessage({ id: 'presentation.rollback.description' })}
            title={intl.formatMessage(
              { id: 'presentation.rollback.confirm' },
              { revision: item.revision },
            )}
            onConfirm={() => rollback.mutate(item.revision)}
          >
            <Button
              disabled={profile.data?.state === 'draft'}
              loading={rollback.isPending && rollback.variables === item.revision}
              size="small"
              type="link"
            >
              {intl.formatMessage({ id: 'presentation.rollback.action' })}
            </Button>
          </Popconfirm>
        ) : null,
    },
  ];

  const subjectRequired = scope !== 'application';
  const createIdentityInvalid =
    creating && (pageKey === '' || (subjectRequired && !subjectID.trim()));
  const detailError = profile.error ?? publishedDocument.error;
  const detailLoading =
    !creating && !detailError && (profile.isPending || (profile.data && !sourceDocument));

  return (
    <Space orientation="vertical" size="middle" className="w-full">
      {recoveryNotice}
      {!capabilityItems.length ? (
        <PageEmpty description={intl.formatMessage({ id: 'presentation.capabilities.empty' })} />
      ) : null}
      {conflict ? (
        <Alert
          action={
            <Popconfirm
              description={intl.formatMessage({ id: 'presentation.conflict.discard.description' })}
              title={intl.formatMessage({ id: 'presentation.conflict.discard.confirm' })}
              onConfirm={discardAndReload}
            >
              <Button size="small">
                {intl.formatMessage({ id: 'presentation.conflict.reload' })}
              </Button>
            </Popconfirm>
          }
          description={intl.formatMessage(
            { id: 'presentation.conflict.description' },
            { version: conflict },
          )}
          showIcon
          title={intl.formatMessage({ id: 'presentation.conflict.title' })}
          type="warning"
        />
      ) : null}
      {localError && !conflict ? (
        <Alert
          closable
          description={localError}
          showIcon
          title={intl.formatMessage({ id: 'presentation.operation.failed' })}
          type="error"
          onClose={() => setLocalError(undefined)}
        />
      ) : null}
      <Card
        title={intl.formatMessage({ id: 'presentation.profiles.title' })}
        extra={
          canDraft && capabilityItems.length ? (
            <Button type="primary" onClick={beginCreate}>
              {intl.formatMessage({ id: 'presentation.create.action' })}
            </Button>
          ) : null
        }
      >
        <Table<PresentationProfileSummary>
          columns={profileColumns}
          dataSource={profiles.data?.items ?? []}
          loading={profiles.isFetching}
          locale={{
            emptyText: (
              <PageEmpty description={intl.formatMessage({ id: 'presentation.profiles.empty' })} />
            ),
          }}
          pagination={{
            current: profilePage,
            showSizeChanger: false,
            total: profiles.data?.total ?? 0,
            onChange: setProfilePage,
          }}
          rowKey="id"
          rowClassName={(item) => (item.id === selectedID ? 'ant-table-row-selected' : '')}
          scroll={{ x: 720 }}
        />
      </Card>

      <Card
        title={
          creating
            ? intl.formatMessage({ id: 'presentation.create.title' })
            : intl.formatMessage({ id: 'presentation.editor.title' })
        }
      >
        {detailError ? (
          <PageError
            message={getRequestErrorMessage(detailError)}
            onRetry={() => {
              if (profile.error) {
                void profile.refetch();
                return;
              }
              void publishedDocument.refetch();
            }}
          />
        ) : detailLoading ? (
          <PageLoading rows={8} />
        ) : (
          <Space orientation="vertical" size="middle" className="w-full">
            {creating ? (
              <Row gutter={16}>
                <Col xs={24} lg={8}>
                  <Typography.Text>
                    {intl.formatMessage({ id: 'presentation.pageKey' })}
                  </Typography.Text>
                  <Select
                    className="w-full"
                    options={capabilityItems.map((item) => ({
                      value: item.pageKey,
                      label: `${capabilityLabel(item, intl.locale)} (${item.pageKey})`,
                    }))}
                    value={pageKey}
                    onChange={(value) => {
                      setPageKey(value);
                      const capability = capabilityItems.find((item) => item.pageKey === value);
                      if (capability) resetTemplate(capability, { ...identity, pageKey: value });
                    }}
                  />
                </Col>
                <Col xs={24} lg={6}>
                  <Typography.Text>
                    {intl.formatMessage({ id: 'presentation.scope' })}
                  </Typography.Text>
                  <Select
                    className="w-full"
                    options={(['application', 'role', 'user'] as const).map((value) => ({
                      value,
                      label: intl.formatMessage({ id: `presentation.scope.${value}` }),
                    }))}
                    value={scope}
                    onChange={(value) => {
                      setScope(value);
                      const nextIdentity = {
                        ...identity,
                        scope: value,
                        subjectID: value === 'application' ? '' : subjectID.trim(),
                      };
                      if (selectedCapability) resetTemplate(selectedCapability, nextIdentity);
                    }}
                  />
                </Col>
                <Col xs={24} lg={10}>
                  <Typography.Text>
                    {intl.formatMessage({ id: 'presentation.subjectID' })}
                  </Typography.Text>
                  <Input
                    disabled={!subjectRequired}
                    maxLength={160}
                    status={subjectRequired && !subjectID.trim() ? 'error' : undefined}
                    value={subjectID}
                    onChange={(event) => setSubjectID(event.target.value)}
                    onBlur={() => {
                      if (selectedCapability && subjectID.trim()) {
                        resetTemplate(selectedCapability, {
                          ...identity,
                          subjectID: subjectID.trim(),
                        });
                      }
                    }}
                  />
                </Col>
              </Row>
            ) : profile.data ? (
              <Descriptions
                column={{ xs: 1, md: 2, lg: 4 }}
                items={[
                  {
                    key: 'page',
                    label: intl.formatMessage({ id: 'presentation.pageKey' }),
                    children: profile.data.pageKey,
                  },
                  {
                    key: 'scope',
                    label: intl.formatMessage({ id: 'presentation.scope' }),
                    children: profile.data.scope,
                  },
                  {
                    key: 'subject',
                    label: intl.formatMessage({ id: 'presentation.subjectID' }),
                    children: profile.data.subjectID || '—',
                  },
                  {
                    key: 'version',
                    label: intl.formatMessage({ id: 'presentation.version' }),
                    children: profile.data.version,
                  },
                ]}
                size="small"
              />
            ) : null}

            <Space wrap>
              <Button loading={validate.isPending} onClick={() => validate.mutate()}>
                {intl.formatMessage({ id: 'presentation.validate.action' })}
              </Button>
              {canDraft ? (
                <Button
                  disabled={
                    createIdentityInvalid ||
                    (!creating && !dirty) ||
                    !editorText ||
                    Boolean(conflict)
                  }
                  loading={save.isPending}
                  type="primary"
                  onClick={() => save.mutate()}
                >
                  {intl.formatMessage({ id: 'presentation.save.action' })}
                </Button>
              ) : null}
              {!creating && canPublish && profile.data?.state === 'draft' ? (
                <Popconfirm
                  description={intl.formatMessage({ id: 'presentation.publish.description' })}
                  title={intl.formatMessage({ id: 'presentation.publish.confirm' })}
                  onConfirm={() => publish.mutate()}
                >
                  <Button
                    disabled={!profile.data.draft?.valid || dirty || Boolean(conflict)}
                    loading={publish.isPending}
                  >
                    {intl.formatMessage({ id: 'presentation.publish.action' })}
                  </Button>
                </Popconfirm>
              ) : null}
              {dirty ? (
                <Tag color="warning">{intl.formatMessage({ id: 'presentation.unsaved' })}</Tag>
              ) : null}
            </Space>

            <Segmented
              aria-label={intl.formatMessage({ id: 'presentation.editor.title' })}
              options={[
                {
                  value: 'visual',
                  label: intl.formatMessage({ id: 'presentation.editor.mode.visual' }),
                },
                {
                  value: 'raw',
                  label: intl.formatMessage({ id: 'presentation.editor.mode.raw' }),
                },
              ]}
              value={editorMode}
              onChange={(mode) => {
                if (mode === 'visual' && !visualDocument) {
                  setLocalError(
                    intl.formatMessage({ id: 'presentation.editor.visual.unavailable' }),
                  );
                  setEditorMode('raw');
                  return;
                }
                setLocalError(undefined);
                setEditorMode(mode as 'raw' | 'visual');
              }}
            />

            {editorMode === 'visual' && visualDocument && selectedCapability ? (
              <PresentationVisualEditor
                capability={selectedCapability}
                document={visualDocument}
                onChange={(document) => {
                  setEditorText(formatPresentationDraftAST(document));
                  setDirty(true);
                  setValidation(undefined);
                  setLocalError(undefined);
                }}
              />
            ) : (
              <Input.TextArea
                id="presentation-profile-json"
                aria-label={intl.formatMessage({ id: 'presentation.document' })}
                autoSize={{ minRows: 18, maxRows: 36 }}
                maxLength={128 * 1024}
                showCount
                style={{ fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace' }}
                value={editorText}
                onChange={(event) => {
                  setEditorText(event.target.value);
                  setDirty(true);
                  setValidation(undefined);
                  setLocalError(undefined);
                }}
              />
            )}

            {validation ? (
              <Alert
                description={
                  validation.issues.length ? (
                    <Space orientation="vertical" size="small">
                      {validation.issues.map((issue) => (
                        <Space key={`${issue.path}:${issue.code}`} wrap>
                          <Typography.Text code>{issue.path}</Typography.Text>
                          <Typography.Text>{`${issue.code}: ${issue.message}`}</Typography.Text>
                          <Button size="small" type="link" onClick={() => openRawIssue(issue.path)}>
                            {intl.formatMessage({ id: 'presentation.validation.openRaw' })}
                          </Button>
                        </Space>
                      ))}
                    </Space>
                  ) : undefined
                }
                showIcon
                title={intl.formatMessage({
                  id:
                    validation.structurallyValid && validation.semanticallyValid
                      ? 'presentation.validation.valid'
                      : 'presentation.validation.invalid',
                })}
                type={
                  validation.structurallyValid && validation.semanticallyValid
                    ? 'success'
                    : 'warning'
                }
              />
            ) : null}
            {preview ? (
              <Card size="small" title={intl.formatMessage({ id: 'presentation.preview.title' })}>
                <Descriptions
                  column={{ xs: 1, md: 2, lg: 4 }}
                  items={[
                    {
                      key: 'title',
                      label: intl.formatMessage({ id: 'presentation.preview.pageTitle' }),
                      children: preview.title,
                    },
                    {
                      key: 'source',
                      label: intl.formatMessage({ id: 'presentation.preview.dataSource' }),
                      children: preview.dataSource,
                    },
                    {
                      key: 'density',
                      label: intl.formatMessage({ id: 'presentation.preview.density' }),
                      children: preview.list.density,
                    },
                    {
                      key: 'pageSize',
                      label: intl.formatMessage({ id: 'presentation.preview.pageSize' }),
                      children: preview.list.pageSize,
                    },
                    {
                      key: 'columns',
                      label: intl.formatMessage({ id: 'presentation.preview.columns' }),
                      children: (
                        <Space wrap>
                          {preview.list.columns.map((field) => (
                            <Tag key={field.field}>{field.label || field.field}</Tag>
                          ))}
                        </Space>
                      ),
                    },
                    {
                      key: 'actions',
                      label: intl.formatMessage({ id: 'presentation.preview.actions' }),
                      children: preview.actions.length ? (
                        <Space wrap>
                          {preview.actions.map((action) => (
                            <Tag key={action.action}>{action.label || action.action}</Tag>
                          ))}
                        </Space>
                      ) : (
                        '—'
                      ),
                    },
                  ]}
                  size="small"
                />
              </Card>
            ) : null}
          </Space>
        )}
      </Card>

      {!creating && selectedID ? (
        <Card title={intl.formatMessage({ id: 'presentation.history.title' })}>
          {revisions.isError ? (
            <PageError
              message={getRequestErrorMessage(revisions.error)}
              onRetry={() => void revisions.refetch()}
            />
          ) : (
            <Table<PresentationRevisionSummary>
              columns={historyColumns}
              dataSource={revisions.data?.items ?? []}
              loading={revisions.isPending || revisions.isFetching}
              locale={{
                emptyText: (
                  <PageEmpty
                    description={intl.formatMessage({ id: 'presentation.history.empty' })}
                  />
                ),
              }}
              pagination={{
                current: historyPage,
                showSizeChanger: false,
                total: revisions.data?.total ?? 0,
                onChange: setHistoryPage,
              }}
              rowKey="revision"
              scroll={{ x: 720 }}
            />
          )}
        </Card>
      ) : null}
    </Space>
  );
}
