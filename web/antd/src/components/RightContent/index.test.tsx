import { render, screen } from '@testing-library/react';
import { Question } from './index';

const React = require('react');

jest.mock('@umijs/max', () => ({
  SelectLang: () => null,
  useIntl: () => ({ formatMessage: () => 'Documentation' }),
}));

describe('header documentation action', () => {
  it('uses a keyboard-accessible external link', () => {
    render(<Question />);

    expect(screen.getByRole('link', { name: 'Documentation' }).getAttribute('href')).toBe(
      'https://docs.mss-boot-io.top',
    );
  });
});
