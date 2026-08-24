import { queryKeys } from '@mss-admin-core/shared/query/client';
import { useQuery } from '@tanstack/react-query';
import { presentationAPI } from './api';

export function usePresentationCapabilities() {
  return useQuery({
    queryKey: queryKeys.presentationCapabilities,
    queryFn: () => presentationAPI.capabilities(),
    staleTime: 60_000,
  });
}

export function usePresentationProfiles() {
  return useQuery({
    queryKey: queryKeys.presentationProfiles,
    queryFn: () => presentationAPI.profiles(),
    staleTime: 15_000,
  });
}

export function usePresentationProfile(id?: string) {
  return useQuery({
    queryKey: queryKeys.presentationProfile(id ?? ''),
    queryFn: () => presentationAPI.profile(id as string),
    enabled: Boolean(id),
    staleTime: 15_000,
  });
}

export function usePresentationRevisions(profileID?: string) {
  return useQuery({
    queryKey: queryKeys.presentationRevisions(profileID ?? ''),
    queryFn: () => presentationAPI.revisions(profileID as string),
    enabled: Boolean(profileID),
    staleTime: 15_000,
  });
}

export function usePresentationPublishedRevision(
  profileID: string | undefined,
  revision: number | undefined,
) {
  return useQuery({
    queryKey: queryKeys.presentationRevision(profileID ?? '', revision ?? 0),
    queryFn: () => presentationAPI.revision(profileID as string, revision as number),
    enabled: Boolean(profileID && revision && revision > 0),
    staleTime: Number.POSITIVE_INFINITY,
  });
}
