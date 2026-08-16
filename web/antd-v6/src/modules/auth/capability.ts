import type { ApplicationProfile } from '@/shared/theme/contract';

export type EmailChallengeFlow = 'login' | 'register' | 'resetPassword';

function enabled(value: unknown): boolean {
  return value === true || value === 'true';
}

export interface EmailChallengeCapability {
  emailEnabled: boolean;
  ready: boolean;
  registerEnabled: boolean;
}

export function emailChallengeCapability(
  profile: ApplicationProfile | undefined,
): EmailChallengeCapability {
  return {
    emailEnabled: enabled(profile?.security.emailEnabled),
    ready: enabled(profile?.security.emailChallengeReady),
    registerEnabled: enabled(profile?.security.registerEnabled),
  };
}

export function isEmailChallengeFlowAvailable(
  profile: ApplicationProfile | undefined,
  flow: EmailChallengeFlow,
): boolean {
  const capability = emailChallengeCapability(profile);
  if (!capability.emailEnabled || !capability.ready) return false;
  return flow !== 'register' || capability.registerEnabled;
}
