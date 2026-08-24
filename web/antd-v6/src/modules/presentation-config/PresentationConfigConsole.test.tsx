import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import PresentationConfigConsole from './PresentationConfigConsole';

const definitionHash = `sha256:${'a'.repeat(64)}`;
const contentDigest = `sha256:${'b'.repeat(64)}`;
const document = {
  apiVersion: 'mss.io/v1alpha1',
  kind: 'AdminPagePresentation',
  metadata: {
    name: 'orders-application',
    pageKey: 'orders.list',
    definitionHash,
    scope: { kind: 'application' },
  },
  spec: { title: { 'en-US': 'Orders' } },
};
const capability = {
  pageKey: 'orders.list',
  definitionVersion: '1',
  definitionHash,
  components: ['text'],
  fields: [],
  dataSources: ['orders.list'],
  actions: [],
  defaultPresentation: { title: { 'en-US': 'Orders' } },
  definition: {
    pageKey: 'orders.list',
    definitionVersion: '1',
    definitionHash,
    components: [{ id: 'text' }],
    fields: [],
    dataSources: [{ id: 'orders.list', requiredPermissions: ['/orders'] }],
    actions: [],
    defaultPresentation: {
      title: { 'en-US': 'Orders' },
      dataSource: 'orders.list',
      list: { columns: [], density: 'middle', pageSize: 20, defaultSort: [] },
      search: { fields: [], collapsedByDefault: false },
      form: { fields: [], columns: 1 },
      detail: { fields: [], columns: 1 },
      actions: [],
    },
  },
};
const draftProfile = {
  id: 'profile-1',
  scope: 'application',
  subjectID: '',
  pageKey: 'orders.list',
  state: 'draft',
  version: 2,
  draftValid: true,
  publishedRevision: 0,
  createdBy: 'author-1',
  updatedBy: 'author-2',
  createdAt: '2026-08-24T00:00:00Z',
  updatedAt: '2026-08-24T00:01:00Z',
  draft: { document, digest: contentDigest, definitionHash, valid: true, issues: [] },
};
const revision = {
  revision: 1,
  aggregateVersion: 3,
  contentDigest,
  definitionHash,
  transition: 'publish',
  actorID: 'publisher-1',
  createdAt: '2026-08-24T00:02:00Z',
  profileID: 'profile-1',
  document,
};
const publishedProfile = {
  ...draftProfile,
  state: 'published',
  version: 3,
  draftValid: undefined,
  draft: undefined,
  publishedRevision: 1,
  updatedAt: '2026-08-24T00:02:00Z',
  published: revision,
};

function queryResult(data: unknown, overrides: Record<string, unknown> = {}) {
  return {
    data,
    error: null,
    isError: false,
    isFetching: false,
    isPending: false,
    refetch: vi.fn(async () => ({ data })),
    ...overrides,
  };
}

const runtime = vi.hoisted(() => ({
  capabilities: {} as ReturnType<typeof queryResult>,
  profiles: {} as ReturnType<typeof queryResult>,
  profile: {} as ReturnType<typeof queryResult>,
  revisions: {} as ReturnType<typeof queryResult>,
  published: {} as ReturnType<typeof queryResult>,
  api: {
    validate: vi.fn(),
    createDraft: vi.fn(),
    replaceDraft: vi.fn(),
    publish: vi.fn(),
    rollback: vi.fn(),
  },
}));

vi.mock('@umijs/max', () => ({
  useIntl: () => ({
    locale: 'en-US',
    formatMessage: ({ id }: { id: string }) => id,
  }),
}));

vi.mock('./api', () => ({
  newPresentationIdempotencyKey: (action: string) => `${action}:test-key`,
  presentationAPI: runtime.api,
}));

vi.mock('./query', () => ({
  usePresentationCapabilities: () => runtime.capabilities,
  usePresentationProfiles: () => runtime.profiles,
  usePresentationProfile: () => runtime.profile,
  usePresentationRevisions: () => runtime.revisions,
  usePresentationPublishedRevision: () => runtime.published,
}));

function renderConsole(props = { canDraft: true, canPublish: true, canRollback: true }) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <App>
        <PresentationConfigConsole {...props} />
      </App>
    </QueryClientProvider>,
  );
}

describe('presentation configuration console', () => {
  beforeEach(() => {
    runtime.capabilities = queryResult({ items: [capability], recoveryMode: false });
    runtime.profiles = queryResult({ items: [draftProfile], page: 1, pageSize: 100, total: 1 });
    runtime.profile = queryResult(draftProfile);
    runtime.revisions = queryResult({ items: [], page: 1, pageSize: 100, total: 0 });
    runtime.published = queryResult(undefined);
    for (const operation of Object.values(runtime.api)) operation.mockReset();
  });

  it('renders explicit empty and recovery states', async () => {
    runtime.capabilities = queryResult({ items: [], recoveryMode: false });
    const empty = renderConsole();
    expect(screen.getByText('presentation.capabilities.empty')).toBeTruthy();
    empty.unmount();

    runtime.capabilities = queryResult({ items: [capability], recoveryMode: true });
    runtime.profiles = queryResult({ items: [], page: 1, pageSize: 100, total: 0 });
    runtime.profile = queryResult(undefined);
    renderConsole();
    expect(screen.getByText('presentation.recovery.title')).toBeTruthy();
    await waitFor(() => expect(screen.getByLabelText('presentation.document')).toBeTruthy());
  });

  it('shows validation diagnostics without mutating the draft', async () => {
    runtime.api.validate.mockResolvedValue({
      structurallyValid: true,
      semanticallyValid: false,
      issues: [{ code: 'unknown-field', path: 'spec.list.columns[0]', message: 'Unknown field' }],
    });
    renderConsole();
    await waitFor(() => expect(screen.getByLabelText('presentation.document')).toBeTruthy());
    fireEvent.click(screen.getByText('presentation.validate.action'));

    await waitFor(() => expect(screen.getByText('presentation.validation.invalid')).toBeTruthy());
    expect(screen.getByText('unknown-field: Unknown field')).toBeTruthy();
    expect(runtime.api.replaceDraft).not.toHaveBeenCalled();
  });

  it('preserves local JSON and reports the current version after a 412 conflict', async () => {
    const current = {
      ...draftProfile,
      version: 3,
      updatedAt: '2026-08-24T00:02:00Z',
    };
    runtime.api.replaceDraft.mockRejectedValue({
      message: 'presentation profile changed since it was loaded',
      response: {
        status: 412,
        data: {
          errorCode: 'PRESENTATION_REVISION_CONFLICT',
          data: { current },
        },
      },
    });
    renderConsole();
    const editor = (await screen.findByLabelText('presentation.document')) as HTMLTextAreaElement;
    const localDocument = JSON.stringify(
      { ...document, spec: { title: { 'en-US': 'My local edit' } } },
      null,
      2,
    );
    fireEvent.change(editor, { target: { value: localDocument } });
    fireEvent.click(screen.getByText('presentation.save.action'));

    await waitFor(() => expect(screen.getByText('presentation.conflict.title')).toBeTruthy());
    expect(editor.value).toBe(localDocument);
    expect(runtime.api.replaceDraft).toHaveBeenCalledTimes(1);
  });

  it('publishes only after confirmation and keeps immutable history visible', async () => {
    runtime.revisions = queryResult({ items: [revision], page: 1, pageSize: 100, total: 1 });
    runtime.api.publish.mockResolvedValue({
      profile: publishedProfile,
      revision,
      replayed: false,
    });
    renderConsole();
    await screen.findByLabelText('presentation.document');
    expect(screen.getByText('publisher-1')).toBeTruthy();
    fireEvent.click(screen.getByText('presentation.publish.action'));
    expect(await screen.findByText('presentation.publish.confirm')).toBeTruthy();
    fireEvent.click(screen.getByText('OK'));

    await waitFor(() => expect(runtime.api.publish).toHaveBeenCalledTimes(1));
  });

  it('rolls back a published profile only after explicit confirmation', async () => {
    runtime.profiles = queryResult({
      items: [publishedProfile],
      page: 1,
      pageSize: 100,
      total: 1,
    });
    runtime.profile = queryResult(publishedProfile);
    runtime.published = queryResult(revision);
    runtime.revisions = queryResult({ items: [revision], page: 1, pageSize: 100, total: 1 });
    runtime.api.rollback.mockResolvedValue({
      profile: { ...publishedProfile, version: 4, publishedRevision: 2 },
      revision: { ...revision, revision: 2, transition: 'rollback', sourceRevision: 1 },
      replayed: false,
    });
    renderConsole();
    const rollbackAction = await screen.findByText('presentation.rollback.action');
    fireEvent.click(rollbackAction);
    expect(await screen.findByText('presentation.rollback.confirm')).toBeTruthy();
    fireEvent.click(screen.getByText('OK'));

    await waitFor(() =>
      expect(runtime.api.rollback).toHaveBeenCalledWith(publishedProfile, 1, 'rollback:test-key'),
    );
  });
});
