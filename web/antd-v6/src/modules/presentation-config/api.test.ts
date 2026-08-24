import { describe, expect, it, vi } from 'vitest';
import {
  createPresentationAPI,
  newPresentationIdempotencyKey,
  type PresentationRequestOptions,
} from './api';
import { parsePresentationProfile } from './contract';

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
const rawProfile = {
  id: 'profile/1',
  scope: 'application',
  pageKey: 'orders.list',
  state: 'draft',
  version: 2,
  draftValid: true,
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
  profileID: 'profile/1',
  document,
};

describe('presentation configuration API', () => {
  it('sends exact create, update, publish, and rollback preconditions', async () => {
    const client = vi.fn(async (path: string, _options: PresentationRequestOptions) => {
      if (path.endsWith('/publish') || path.endsWith('/rollback')) {
        return { profile: rawProfile, revision, replayed: false };
      }
      return rawProfile;
    });
    const api = createPresentationAPI(client);
    const profile = parsePresentationProfile(rawProfile);

    await api.createDraft(
      { scope: 'application', subjectID: '', pageKey: 'orders.list' },
      document,
    );
    await api.replaceDraft(profile, document);
    await api.publish(profile, 'publish-key-1');
    await api.rollback(profile, 1, 'rollback-key-1');

    expect(client).toHaveBeenNthCalledWith(1, '/presentation-profiles', {
      method: 'POST',
      data: { scope: 'application', pageKey: 'orders.list', document },
      headers: { 'If-None-Match': '*' },
      skipErrorHandler: true,
    });
    expect(client).toHaveBeenNthCalledWith(2, '/presentation-profiles/profile%2F1/draft', {
      method: 'PUT',
      data: { document },
      headers: { 'If-Match': '"presentation-profile-profile/1-2"' },
      skipErrorHandler: true,
    });
    expect(client).toHaveBeenNthCalledWith(3, '/presentation-profiles/profile%2F1/publish', {
      method: 'POST',
      headers: {
        'Idempotency-Key': 'publish-key-1',
        'If-Match': '"presentation-profile-profile/1-2"',
      },
      skipErrorHandler: true,
    });
    expect(client).toHaveBeenNthCalledWith(4, '/presentation-profiles/profile%2F1/rollback', {
      method: 'POST',
      data: { revision: 1 },
      headers: {
        'Idempotency-Key': 'rollback-key-1',
        'If-Match': '"presentation-profile-profile/1-2"',
      },
      skipErrorHandler: true,
    });
  });

  it('uses bounded collection queries and encoded history paths', async () => {
    const client = vi.fn(async (path: string) => {
      if (path === '/presentation-profiles') {
        return { items: [], page: 1, pageSize: 100, total: 0 };
      }
      if (path.endsWith('/revisions')) {
        return { items: [], page: 1, pageSize: 100, total: 0 };
      }
      return revision;
    });
    const api = createPresentationAPI(client);

    await api.profiles(3, 50);
    await api.revisions('profile/1', 4, 20);
    await api.revision('profile/1', 1);

    expect(client).toHaveBeenNthCalledWith(1, '/presentation-profiles', {
      method: 'GET',
      params: { page: 3, pageSize: 50 },
      skipErrorHandler: true,
    });
    expect(client).toHaveBeenNthCalledWith(2, '/presentation-profiles/profile%2F1/revisions', {
      method: 'GET',
      params: { page: 4, pageSize: 20 },
      skipErrorHandler: true,
    });
    expect(client).toHaveBeenNthCalledWith(3, '/presentation-profiles/profile%2F1/revisions/1', {
      method: 'GET',
      skipErrorHandler: true,
    });
  });

  it('creates action-specific unique idempotency keys', () => {
    const first = newPresentationIdempotencyKey('publish');
    const second = newPresentationIdempotencyKey('publish');
    expect(first).toMatch(/^publish:[0-9a-f-]{36}$/);
    expect(second).not.toBe(first);
    expect(newPresentationIdempotencyKey('rollback')).toMatch(/^rollback:/);
  });
});
