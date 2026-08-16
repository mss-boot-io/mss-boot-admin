import { request } from '@umijs/max';
import type { EmailChallengeFlow } from './capability';

export interface EmailChallengeRequest {
  email: string;
  useBy: EmailChallengeFlow;
}

export interface PasswordRecoveryRequest {
  email: string;
  captcha: string;
  password: string;
}

export const authAPI = {
  sendEmailChallenge: async (input: EmailChallengeRequest): Promise<void> => {
    await request('/user/fakeCaptcha', {
      method: 'POST',
      data: input,
      skipErrorHandler: true,
    });
  },
  resetPassword: async (input: PasswordRecoveryRequest): Promise<void> => {
    await request('/user/reset-password', {
      method: 'POST',
      data: input,
      skipErrorHandler: true,
    });
  },
};
