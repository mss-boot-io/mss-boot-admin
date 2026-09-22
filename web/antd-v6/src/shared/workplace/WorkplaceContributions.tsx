import entries from '@mss-admin-business/workplace';
import { hasPermission } from '@mss-admin-core/shared/auth/access';
import type { InitialState } from '@mss-admin-core/shared/auth/types';
import { useIntl, useModel } from '@umijs/max';
import { Alert } from 'antd';
import { Component, type ReactNode } from 'react';
import { defineWorkplaceContributions, type WorkplaceContribution } from './contract';

class ContributionBoundary extends Component<
  { children: ReactNode; fallback: ReactNode },
  { failed: boolean }
> {
  state = { failed: false };
  static getDerivedStateFromError() {
    return { failed: true };
  }
  render() {
    return this.state.failed ? this.props.fallback : this.props.children;
  }
}

const registered = defineWorkplaceContributions(entries);

export function WorkplaceContributions({
  contributions = registered,
}: {
  contributions?: readonly WorkplaceContribution[];
}) {
  const intl = useIntl();
  const { initialState } = useModel('@@initialState') as { initialState?: InitialState };
  const user = initialState?.currentUser;
  if (!user) return null;
  return (
    <>
      {contributions
        .filter((item) => hasPermission(user, item.permission))
        .map((item) => {
          const BusinessContent = item.component;
          return (
            <div className="mb-4" key={`${user.id}:${item.id}`}>
              <ContributionBoundary
                fallback={
                  <Alert
                    type="error"
                    showIcon
                    title={intl.formatMessage({ id: 'pages.workplace.businessUnavailable' })}
                  />
                }
              >
                <BusinessContent currentUser={user} />
              </ContributionBoundary>
            </div>
          );
        })}
    </>
  );
}
