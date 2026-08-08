import { buildTreeOptions, findTreeOptionLabel } from './treeOptions';

describe('user tree options', () => {
  it('keeps the edited node disabled through nested branches', () => {
    const options = buildTreeOptions(
      [
        {
          id: 'root',
          name: 'Root',
          children: [{ id: 'self', name: 'Current node' }],
        },
      ],
      'self',
    );

    expect(options[0].children?.[0]).toMatchObject({ value: 'self', disabled: true });
  });

  it('returns a match from an earlier sibling without a later branch overwriting it', () => {
    const options = buildTreeOptions([
      { id: 'first', name: 'First branch' },
      {
        id: 'second',
        name: 'Second branch',
        children: [{ id: 'nested', name: 'Nested branch' }],
      },
    ]);

    expect(findTreeOptionLabel(options, 'first')).toBe('First branch');
    expect(findTreeOptionLabel(options, 'nested')).toBe('Nested branch');
    expect(findTreeOptionLabel(options, 'missing')).toBe('');
  });
});
