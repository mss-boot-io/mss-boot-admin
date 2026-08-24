import { request } from '@umijs/max';
import {
  type PresentationCapabilityCatalog,
  type PresentationJSONObject,
  type PresentationProfile,
  type PresentationProfileIdentity,
  type PresentationProfilePage,
  type PresentationRevision,
  type PresentationRevisionPage,
  type PresentationTransitionResult,
  type PresentationValidationResult,
  parsePresentationCapabilityCatalog,
  parsePresentationProfile,
  parsePresentationProfilePage,
  parsePresentationRevision,
  parsePresentationRevisionPage,
  parsePresentationTransition,
  parsePresentationValidation,
  presentationProfileETag,
} from './contract';

export interface PresentationRequestOptions {
  method: 'GET' | 'POST' | 'PUT';
  data?: unknown;
  headers?: Record<string, string>;
  params?: Record<string, number | string | undefined>;
  skipErrorHandler: true;
}

export type PresentationRequestClient = (
  path: string,
  options: PresentationRequestOptions,
) => Promise<unknown>;

export function createPresentationAPI(client: PresentationRequestClient) {
  return {
    capabilities: async (): Promise<PresentationCapabilityCatalog> =>
      parsePresentationCapabilityCatalog(
        await client('/presentation-capabilities', {
          method: 'GET',
          skipErrorHandler: true,
        }),
      ),

    profiles: async (page: number, pageSize: number): Promise<PresentationProfilePage> =>
      parsePresentationProfilePage(
        await client('/presentation-profiles', {
          method: 'GET',
          params: { page, pageSize },
          skipErrorHandler: true,
        }),
      ),

    profile: async (id: string): Promise<PresentationProfile> =>
      parsePresentationProfile(
        await client(`/presentation-profiles/${encodeURIComponent(id)}`, {
          method: 'GET',
          skipErrorHandler: true,
        }),
      ),

    validate: async (document: PresentationJSONObject): Promise<PresentationValidationResult> =>
      parsePresentationValidation(
        await client('/presentation-profiles/validate', {
          method: 'POST',
          data: { document },
          skipErrorHandler: true,
        }),
      ),

    createDraft: async (
      identity: PresentationProfileIdentity,
      document: PresentationJSONObject,
    ): Promise<PresentationProfile> =>
      parsePresentationProfile(
        await client('/presentation-profiles', {
          method: 'POST',
          data: {
            scope: identity.scope,
            ...(identity.subjectID ? { subjectID: identity.subjectID } : {}),
            pageKey: identity.pageKey,
            document,
          },
          headers: { 'If-None-Match': '*' },
          skipErrorHandler: true,
        }),
      ),

    replaceDraft: async (
      profile: PresentationProfile,
      document: PresentationJSONObject,
    ): Promise<PresentationProfile> =>
      parsePresentationProfile(
        await client(`/presentation-profiles/${encodeURIComponent(profile.id)}/draft`, {
          method: 'PUT',
          data: { document },
          headers: { 'If-Match': presentationProfileETag(profile) },
          skipErrorHandler: true,
        }),
      ),

    publish: async (
      profile: PresentationProfile,
      idempotencyKey: string,
    ): Promise<PresentationTransitionResult> =>
      parsePresentationTransition(
        await client(`/presentation-profiles/${encodeURIComponent(profile.id)}/publish`, {
          method: 'POST',
          headers: {
            'Idempotency-Key': idempotencyKey,
            'If-Match': presentationProfileETag(profile),
          },
          skipErrorHandler: true,
        }),
      ),

    revisions: async (
      profileID: string,
      page: number,
      pageSize: number,
    ): Promise<PresentationRevisionPage> =>
      parsePresentationRevisionPage(
        await client(`/presentation-profiles/${encodeURIComponent(profileID)}/revisions`, {
          method: 'GET',
          params: { page, pageSize },
          skipErrorHandler: true,
        }),
      ),

    revision: async (profileID: string, revision: number): Promise<PresentationRevision> =>
      parsePresentationRevision(
        await client(
          `/presentation-profiles/${encodeURIComponent(profileID)}/revisions/${revision}`,
          {
            method: 'GET',
            skipErrorHandler: true,
          },
        ),
      ),

    rollback: async (
      profile: PresentationProfile,
      revision: number,
      idempotencyKey: string,
    ): Promise<PresentationTransitionResult> =>
      parsePresentationTransition(
        await client(`/presentation-profiles/${encodeURIComponent(profile.id)}/rollback`, {
          method: 'POST',
          data: { revision },
          headers: {
            'Idempotency-Key': idempotencyKey,
            'If-Match': presentationProfileETag(profile),
          },
          skipErrorHandler: true,
        }),
      ),
  };
}

export function newPresentationIdempotencyKey(action: 'publish' | 'rollback'): string {
  return `${action}:${crypto.randomUUID()}`;
}

export const presentationAPI = createPresentationAPI((path, options) =>
  request<unknown>(path, options),
);
