import { EmailChallengeRoute } from '@/pages/User/Login/emailChallengeAccess';
import { PUBLIC_ROUTE_PATHS } from '@/utils/routeAccess';
import { history } from '@umijs/max';
import React from 'react';

const EmailChallengeRouteWrapper: React.FC<{ children: React.ReactNode }> = ({ children }) => (
  <EmailChallengeRoute
    flow={
      history.location.pathname === PUBLIC_ROUTE_PATHS.register ? 'register' : 'resetPassword'
    }
  >
    {children}
  </EmailChallengeRoute>
);

export default EmailChallengeRouteWrapper;
