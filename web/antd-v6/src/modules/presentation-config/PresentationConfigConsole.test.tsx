import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import PresentationConfigConsole from './PresentationConfigConsole';

const definitionHash = `sha256:${'a'.repeat(64)}`;
const contentDigest = `sha256:${'b'.repeat(64)}`;
const customerDefinitionHash = `sha256:${'c'.repeat(64)}`;
const customerContentDigest = `sha256:${'d'.repeat(64)}`;
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
const customerDocument = {
  ...document,
  metadata: {
    ...document.metadata,
    name: 'customers-application',
    pageKey: 'customers.list',
    definitionHash: customerDefinitionHash,
  },
  spec: { title: { 'en-US': 'Customers' } },
};
const customerCapability = {
  ...capability,
  pageKey: 'customers.list',
  definitionHash: customerDefinitionHash,
  dataSources: ['customers.list'],
  defaultPresentation: { title: { 'en-US': 'Customers' } },
  definition: {
    ...capability.definition,
    pageKey: 'customers.list',
    definitionHash: customerDefinitionHash,
    dataSources: [{ id: 'customers.list', requiredPermissions: ['/customers'] }],
    defaultPresentation: {
      ...capability.definition.defaultPresentation,
      title: { 'en-US': 'Customers' },
      dataSource: 'customers.list',
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
const customerProfile = {
  ...draftProfile,
  id: 'profile-2',
  pageKey: 'customers.list',
  draft: {
    document: customerDocument,
    digest: customerContentDigest,
    definitionHash: customerDefinitionHash,
    valid: true,
    issues: [],
  },
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
  profilesByID: {} as Record<string, ReturnType<typeof queryResult>>,
  revisions: {} as ReturnType<typeof queryResult>,
  published: {} as ReturnType<typeof queryResult>,
  profileQuery: vi.fn(),
  revisionQuery: vi.fn(),
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
  usePresentationProfiles: (page: number, pageSize: number) => {
    runtime.profileQuery(page, pageSize);
    return runtime.profiles;
  },
  usePresentationProfile: (id?: string) =>
    (id ? runtime.profilesByID[id] : undefined) ?? runtime.profile,
  usePresentationRevisions: (profileID?: string, page?: number, pageSize?: number) => {
    runtime.revisionQuery(profileID, page, pageSize);
    return runtime.revisions;
  },
  usePresentationPublishedRevision: () => runtime.published,
}));

function renderConsole(props = { canDraft: true, canPublish: true, canRollback: true }) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const element = (
    <QueryClientProvider client={client}>
      <App>
        <PresentationConfigConsole {...props} />
      </App>
    </QueryClientProvider>
  );
  const view = render(element);
  return { ...view, rerenderConsole: () => view.rerender(element) };
}

describe('presentation configuration console', () => {
  beforeEach(() => {
    runtime.capabilities = queryResult({
      items: [capability],
      recoveryMode: false,
      adoptionMode: 'active',
      activePages: ['orders.list'],
    });
    runtime.profiles = queryResult({ items: [draftProfile], page: 1, pageSize: 100, total: 1 });
    runtime.profile = queryResult(draftProfile);
    runtime.profilesByID = {};
    runtime.revisions = queryResult({ items: [], page: 1, pageSize: 100, total: 0 });
    runtime.published = queryResult(undefined);
    runtime.profileQuery.mockReset();
    runtime.revisionQuery.mockReset();
    for (const operation of Object.values(runtime.api)) operation.mockReset();
  });

  it('renders explicit empty and recovery states', async () => {
    runtime.capabilities = queryResult({
      items: [],
      recoveryMode: false,
      adoptionMode: 'disabled',
      activePages: [],
    });
    const empty = renderConsole();
    expect(screen.getByText('presentation.capabilities.empty')).toBeTruthy();
    expect(screen.getByText('presentation.adoption.disabled.title')).toBeTruthy();
    empty.unmount();

    runtime.capabilities = queryResult({
      items: [],
      recoveryMode: true,
      adoptionMode: 'active',
      activePages: ['retired.list'],
    });
    runtime.profiles = queryResult({ items: [draftProfile], page: 1, pageSize: 100, total: 1 });
    renderConsole();
    expect(screen.getByText('presentation.recovery.title')).toBeTruthy();
    expect(screen.getByText('presentation.adoption.active.title')).toBeTruthy();
    expect(screen.getByText('retired.list')).toBeTruthy();
    expect(screen.getByText('presentation.profiles.title')).toBeTruthy();
    await waitFor(() => expect(screen.getByLabelText('presentation.document')).toBeTruthy());
  });

  it('distinguishes active, shadow, and disabled runtime adoption', () => {
    const active = renderConsole();
    expect(screen.getByText('presentation.adoption.active.title')).toBeTruthy();
    expect(screen.getByText('presentation.adoption.activePages')).toBeTruthy();
    active.unmount();

    runtime.capabilities = queryResult({
      items: [capability],
      recoveryMode: false,
      adoptionMode: 'shadow',
      activePages: ['orders.list'],
    });
    const shadow = renderConsole();
    expect(screen.getByText('presentation.adoption.shadow.title')).toBeTruthy();
    shadow.unmount();

    runtime.capabilities = queryResult({
      items: [capability],
      recoveryMode: false,
      adoptionMode: 'disabled',
      activePages: [],
    });
    renderConsole();
    expect(screen.getByText('presentation.adoption.disabled.title')).toBeTruthy();
    expect(screen.getByText('presentation.adoption.activePages.empty')).toBeTruthy();
  });

  it('waits for the profile page before deciding whether to create a draft', async () => {
    runtime.profiles = queryResult(
      { items: [], page: 1, pageSize: 10, total: 0 },
      { isFetching: true, isPending: true },
    );
    const view = renderConsole();

    runtime.profiles = queryResult({ items: [customerProfile], page: 1, pageSize: 10, total: 1 });
    view.rerenderConsole();

    expect(await screen.findByText('presentation.editor.title')).toBeTruthy();
    expect(screen.queryByText('presentation.create.title')).toBeNull();
  });

  it('uses the selected profile capability when building a preview', async () => {
    runtime.capabilities = queryResult({
      items: [capability, customerCapability],
      recoveryMode: false,
      adoptionMode: 'active',
      activePages: ['orders.list', 'customers.list'],
    });
    runtime.profiles = queryResult({
      items: [draftProfile, customerProfile],
      page: 1,
      pageSize: 20,
      total: 2,
    });
    runtime.profilesByID = {
      'profile-1': queryResult(draftProfile),
      'profile-2': queryResult(customerProfile),
    };
    runtime.api.validate.mockResolvedValue({
      structurallyValid: true,
      semanticallyValid: true,
      canonicalDocument: customerDocument,
      digest: customerContentDigest,
      currentDefinition: customerDefinitionHash,
      issues: [],
    });

    renderConsole();
    fireEvent.click(await screen.findByText('customers.list · application'));
    const editor = (await screen.findByLabelText('presentation.document')) as HTMLTextAreaElement;
    await waitFor(() => expect(editor.value).toContain('customers.list'));
    fireEvent.click(screen.getByText('presentation.validate.action'));

    await waitFor(() => expect(screen.getByText('presentation.preview.title')).toBeTruthy());
  });

  it('requests later profile and history pages from table pagination', async () => {
    runtime.profiles = queryResult({ items: [draftProfile], page: 1, pageSize: 20, total: 21 });
    runtime.revisions = queryResult({ items: [revision], page: 1, pageSize: 20, total: 21 });

    renderConsole();
    const secondPageButtons = await waitFor(() => {
      const buttons = screen.getAllByTitle('2');
      expect(buttons).toHaveLength(2);
      return buttons;
    });
    const profileSecondPageButton = secondPageButtons.at(0);
    expect(profileSecondPageButton).toBeTruthy();
    if (!profileSecondPageButton) {
      throw new Error('profile pagination button is missing');
    }
    fireEvent.click(profileSecondPageButton);
    await waitFor(() => expect(runtime.profileQuery).toHaveBeenCalledWith(2, 10));

    const refreshedSecondPageButtons = screen.getAllByTitle('2');
    const historySecondPageButton = refreshedSecondPageButtons.at(-1);
    expect(historySecondPageButton).toBeTruthy();
    if (!historySecondPageButton) {
      throw new Error('history pagination button is missing');
    }
    fireEvent.click(historySecondPageButton);
    await waitFor(() => expect(runtime.revisionQuery).toHaveBeenCalledWith('profile-1', 2, 10));
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

  it('round-trips between raw and visual modes through one lossless draft AST', async () => {
    renderConsole();
    const editor = (await screen.findByLabelText('presentation.document')) as HTMLTextAreaElement;
    const localDocument = {
      ...document,
      spec: {
        title: { 'en-US': 'Orders' },
        search: { collapsedByDefault: false },
        detail: {
          fields: [
            {
              field: 'status',
              hidden: false,
              visibleWhen: {
                all: [
                  { field: 'status', operator: 'exists' },
                  { not: { field: 'status', operator: 'eq', value: 'closed' } },
                ],
              },
            },
          ],
        },
      },
      futureRoot: { enabled: false },
    };
    fireEvent.change(editor, { target: { value: JSON.stringify(localDocument, null, 2) } });
    fireEvent.click(screen.getByText('presentation.editor.mode.visual'));
    const visualTitle = await screen.findByLabelText('presentation.visual.title en-US');
    fireEvent.change(visualTitle, { target: { value: 'Configured orders' } });
    fireEvent.click(screen.getByText('presentation.editor.mode.raw'));

    const roundTripEditor = (await screen.findByLabelText(
      'presentation.document',
    )) as HTMLTextAreaElement;
    const roundTrip = JSON.parse(roundTripEditor.value);
    expect(roundTrip.spec.title['en-US']).toBe('Configured orders');
    expect(roundTrip.spec.search.collapsedByDefault).toBe(false);
    expect(roundTrip.spec.detail.fields[0].hidden).toBe(false);
    expect(roundTrip.spec.detail.fields[0].visibleWhen.all).toHaveLength(2);
    expect(roundTrip.futureRoot.enabled).toBe(false);
  });

  it('opens the raw editor and focuses it from a validation issue path', async () => {
    runtime.api.validate.mockResolvedValue({
      structurallyValid: true,
      semanticallyValid: false,
      issues: [{ code: 'unknown-field', path: 'spec.list.columns[0].field', message: 'Unknown' }],
    });
    renderConsole();
    await screen.findByLabelText('presentation.document');
    fireEvent.click(screen.getByText('presentation.editor.mode.visual'));
    fireEvent.click(screen.getByText('presentation.validate.action'));
    const locate = await screen.findByText('presentation.validation.openRaw');
    fireEvent.click(locate);

    await waitFor(() => {
      expect(screen.getByLabelText('presentation.document')).toBe(
        globalThis.document.activeElement,
      );
    });
  });

  it('does not retry the disabled published revision query for a draft detail error', async () => {
    const profileRefetch = vi.fn(async () => ({ data: draftProfile }));
    runtime.profile = queryResult(undefined, {
      error: new Error('profile unavailable'),
      isError: true,
      refetch: profileRefetch,
    });
    runtime.published = queryResult(undefined);

    renderConsole();
    fireEvent.click(await screen.findByText('actions.retry'));

    expect(profileRefetch).toHaveBeenCalledTimes(1);
    expect(runtime.published.refetch).not.toHaveBeenCalled();
  });

  it('preserves local JSON and reports the current version after a 412 conflict', async () => {
    const current = {
      id: draftProfile.id,
      version: 3,
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
    fireEvent.click(screen.getByText('presentation.conflict.reload'));
    fireEvent.click(await screen.findByText('OK'));
    await waitFor(() => {
      expect(runtime.profile.refetch).toHaveBeenCalledTimes(1);
      expect(runtime.profiles.refetch).toHaveBeenCalledTimes(1);
    });
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
