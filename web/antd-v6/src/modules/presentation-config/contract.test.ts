import { describe, expect, it } from 'vitest';
import {
  buildInitialPresentationDocument,
  formatPresentationDocument,
  MAX_PRESENTATION_DOCUMENT_BYTES,
  PresentationContractError,
  parsePresentationCapabilityCatalog,
  parsePresentationDocumentText,
  parsePresentationProfile,
  presentationConflictCurrent,
  presentationProfileETag,
} from './contract';

const definitionHash = `sha256:${'a'.repeat(64)}`;
const contentDigest = `sha256:${'b'.repeat(64)}`;

const capability = {
  pageKey: 'orders.list',
  definitionVersion: '1',
  definitionHash,
  components: [{ id: 'text' }],
  fields: [
    {
      id: 'status',
      label: { 'zh-CN': '状态', 'en-US': 'Status' },
      valueType: 'enum',
      required: true,
      sortable: true,
      filterable: true,
      surfaces: ['list', 'search', 'form', 'detail'],
      components: ['text'],
    },
  ],
  dataSources: [{ id: 'orders.list', requiredPermissions: ['/orders'] }],
  actions: [{ id: 'orders.read', requiredPermissions: ['/orders/read'], placements: ['row'] }],
  defaultPresentation: {
    title: { 'zh-CN': '订单', 'en-US': 'Orders' },
    dataSource: 'orders.list',
    list: {
      columns: [{ field: 'status', component: 'text', order: 10, hidden: false }],
      density: 'middle',
      pageSize: 20,
      defaultSort: [{ field: 'status', direction: 'asc' }],
    },
    search: {
      fields: [{ field: 'status', component: 'text', order: 10, hidden: false }],
      collapsedByDefault: false,
    },
    form: {
      fields: [{ field: 'status', component: 'text', order: 10, hidden: false }],
      columns: 1,
    },
    detail: {
      fields: [{ field: 'status', component: 'text', order: 10, hidden: false }],
      columns: 1,
    },
    actions: [{ action: 'orders.read', placement: 'row', order: 10, hidden: false }],
  },
};

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

const draftProfile = {
  id: 'profile-1',
  scope: 'application',
  pageKey: 'orders.list',
  state: 'draft',
  version: 2,
  draftValid: true,
  createdBy: 'author-1',
  updatedBy: 'author-2',
  createdAt: '2026-08-24T00:00:00Z',
  updatedAt: '2026-08-24T00:01:00Z',
  draft: {
    document,
    digest: contentDigest,
    definitionHash,
    valid: true,
    issues: [],
  },
};

describe('presentation configuration contract', () => {
  it('accepts a bounded capability catalog and rejects duplicate page keys', () => {
    expect(
      parsePresentationCapabilityCatalog({ items: [capability], recoveryMode: false }),
    ).toMatchObject({ items: [{ pageKey: 'orders.list' }], recoveryMode: false });
    expect(() =>
      parsePresentationCapabilityCatalog({
        items: [capability, capability],
        recoveryMode: false,
      }),
    ).toThrow(PresentationContractError);
  });

  it('builds scope-safe initial documents and enforces the editor size boundary', () => {
    const catalog = parsePresentationCapabilityCatalog({
      items: [capability],
      recoveryMode: false,
    });
    const parsedCapability = catalog.items[0];
    if (!parsedCapability) throw new Error('capability fixture is missing');
    const application = buildInitialPresentationDocument(parsedCapability, {
      scope: 'application',
      subjectID: 'must-not-leak',
      pageKey: 'orders.list',
    });
    expect(application.metadata).toEqual({
      name: 'orders.list-application',
      pageKey: 'orders.list',
      definitionHash,
      scope: { kind: 'application' },
    });
    const role = buildInitialPresentationDocument(parsedCapability, {
      scope: 'role',
      subjectID: 'role-sales',
      pageKey: 'orders.list',
    });
    expect(role.metadata).toMatchObject({ scope: { kind: 'role', subject: 'role-sales' } });
    expect(parsePresentationDocumentText(formatPresentationDocument(role))).toEqual(role);
    expect(() => parsePresentationDocumentText('{')).toThrow(PresentationContractError);
    expect(() =>
      parsePresentationDocumentText(`{"value":"${'x'.repeat(MAX_PRESENTATION_DOCUMENT_BYTES)}"}`),
    ).toThrow(PresentationContractError);
  });

  it('requires profile state to agree with its draft and derives canonical strong ETags', () => {
    const profile = parsePresentationProfile(draftProfile);
    expect(profile.draft?.document).toEqual(document);
    expect(presentationProfileETag(profile)).toBe('"presentation-profile-profile-1-2"');
    expect(() => parsePresentationProfile({ ...draftProfile, state: 'published' })).toThrow(
      PresentationContractError,
    );
  });

  it('extracts a validated current resource from a 412 response only', () => {
    expect(
      presentationConflictCurrent({
        response: {
          data: {
            errorCode: 'PRESENTATION_REVISION_CONFLICT',
            data: { current: draftProfile },
          },
        },
      }),
    ).toMatchObject({ id: 'profile-1', version: 2 });
    expect(
      presentationConflictCurrent({ response: { data: { data: { current: draftProfile } } } }),
    ).toBeUndefined();
  });
});
