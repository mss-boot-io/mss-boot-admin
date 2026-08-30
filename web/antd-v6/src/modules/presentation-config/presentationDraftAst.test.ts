import { describe, expect, it } from 'vitest';
import {
  formatPresentationDraftAST,
  parsePresentationDraftAST,
  presentationActionOverride,
  presentationDraftSection,
  presentationDraftSpec,
  presentationFieldOverride,
  resetPresentationActionOverrides,
  resetPresentationFieldOverrides,
  setPresentationActionCondition,
  setPresentationActionLocalizedOverride,
  setPresentationActionOverride,
  setPresentationFieldLocalizedOverride,
  setPresentationFieldOverride,
  setPresentationLocalizedOverride,
  setPresentationSpecOverride,
} from './presentationDraftAst';

const definitionHash = `sha256:${'a'.repeat(64)}`;
const source = {
  apiVersion: 'mss.io/v1alpha1',
  kind: 'AdminPagePresentation',
  metadata: {
    name: 'orders-application',
    pageKey: 'orders.list',
    definitionHash,
    scope: { kind: 'application' },
  },
  spec: {
    search: {
      collapsedByDefault: false,
      fields: [
        {
          field: 'status',
          hidden: false,
          visibleWhen: {
            all: [
              { field: 'region', operator: 'exists' },
              { not: { field: 'status', operator: 'eq', value: 'closed' } },
            ],
          },
          futureProperty: false,
        },
      ],
    },
    actions: [
      {
        action: 'orders.read',
        hidden: false,
        visibleWhen: { field: 'status', operator: 'neq', value: 'closed' },
      },
    ],
  },
  futureRoot: { enabled: false },
};

describe('presentation draft AST', () => {
  it('round-trips without dropping false values, compound conditions, unknown data, or omissions', () => {
    const document = parsePresentationDraftAST(JSON.stringify(source));
    const roundTrip = parsePresentationDraftAST(formatPresentationDraftAST(document));

    expect(roundTrip).toEqual(source);
    expect(presentationDraftSection(roundTrip, 'list')).not.toHaveProperty('density');
    expect(presentationDraftSection(roundTrip, 'search').collapsedByDefault).toBe(false);
    expect(presentationFieldOverride(roundTrip, 'search', 'status')).toMatchObject({
      hidden: false,
      futureProperty: false,
      visibleWhen: source.spec.search.fields[0]?.visibleWhen,
    });
  });

  it('changes one visual property while preserving unrelated raw AST nodes', () => {
    const document = parsePresentationDraftAST(JSON.stringify(source));
    const titled = setPresentationLocalizedOverride(
      document,
      undefined,
      'title',
      'en-US',
      'Configured orders',
    );
    const labeled = setPresentationFieldLocalizedOverride(
      titled,
      'search',
      'status',
      'label',
      'zh-CN',
      '订单状态',
    );

    expect(presentationDraftSpec(labeled).title).toEqual({ 'en-US': 'Configured orders' });
    expect(presentationDraftSection(labeled, 'search').collapsedByDefault).toBe(false);
    expect(presentationFieldOverride(labeled, 'search', 'status')).toMatchObject({
      hidden: false,
      label: { 'zh-CN': '订单状态' },
      futureProperty: false,
      visibleWhen: source.spec.search.fields[0]?.visibleWhen,
    });
    expect(labeled.futureRoot).toEqual({ enabled: false });
  });

  it('distinguishes explicit false and empty sort from inheritance', () => {
    const base = parsePresentationDraftAST(
      JSON.stringify({ ...source, spec: { title: { 'en-US': 'Orders' } } }),
    );
    const hidden = setPresentationFieldOverride(base, 'detail', 'status', 'hidden', false);
    const emptySort = setPresentationSpecOverride(hidden, 'list', 'defaultSort', []);

    expect(presentationFieldOverride(emptySort, 'detail', 'status')).toEqual({
      field: 'status',
      hidden: false,
    });
    expect(presentationDraftSection(emptySort, 'list').defaultSort).toEqual([]);

    const inheritedHidden = setPresentationFieldOverride(
      emptySort,
      'detail',
      'status',
      'hidden',
      undefined,
    );
    const inheritedSort = setPresentationSpecOverride(
      inheritedHidden,
      'list',
      'defaultSort',
      undefined,
    );
    expect(presentationFieldOverride(inheritedSort, 'detail', 'status')).toBeUndefined();
    expect(presentationDraftSection(inheritedSort, 'list')).toEqual({});
  });

  it('updates and restores field and action patches without affecting their siblings', () => {
    const document = parsePresentationDraftAST(JSON.stringify(source));
    const withOrder = setPresentationFieldOverride(document, 'search', 'region', 'order', 15);
    const withAction = setPresentationActionLocalizedOverride(
      setPresentationActionOverride(withOrder, 'orders.update', 'hidden', false),
      'orders.update',
      'label',
      'en-US',
      'Edit',
    );
    const conditioned = setPresentationActionCondition(withAction, 'orders.update', {
      field: 'status',
      operator: 'eq',
      value: 'open',
    });

    expect(presentationFieldOverride(conditioned, 'search', 'status')).toMatchObject({
      hidden: false,
    });
    expect(presentationFieldOverride(conditioned, 'search', 'region')).toEqual({
      field: 'region',
      order: 15,
    });
    expect(presentationActionOverride(conditioned, 'orders.update')).toEqual({
      action: 'orders.update',
      hidden: false,
      label: { 'en-US': 'Edit' },
      visibleWhen: { field: 'status', operator: 'eq', value: 'open' },
    });

    const fieldRestored = resetPresentationFieldOverrides(conditioned, 'search', 'region');
    const actionRestored = resetPresentationActionOverrides(fieldRestored, 'orders.update');
    expect(presentationFieldOverride(actionRestored, 'search', 'region')).toBeUndefined();
    expect(presentationFieldOverride(actionRestored, 'search', 'status')).toBeDefined();
    expect(presentationActionOverride(actionRestored, 'orders.update')).toBeUndefined();
    expect(presentationActionOverride(actionRestored, 'orders.read')).toBeDefined();
  });
});
