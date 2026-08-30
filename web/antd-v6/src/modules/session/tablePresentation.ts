import { corePresentationRegistry } from '../../generated/core-presentation-registry.generated';

export const onlineSessionPresentationRegistryEntry =
  corePresentationRegistry['online-session.list'];

export const onlineSessionPresentationListComponents = {
  username: 'online-session-user',
  ip: 'text',
  userAgent: 'online-session-device',
  lastSeenAt: 'date-time',
  status: 'online-session-status',
} as const;

export const onlineSessionPresentationSearchComponents = {
  username: 'input',
  ip: 'input',
  status: 'status-filter',
} as const;

export const onlineSessionPresentationMobileFields = [
  'username',
  'ip',
  'lastSeenAt',
  'status',
] as const;
