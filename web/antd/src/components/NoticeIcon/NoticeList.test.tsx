import { fireEvent, render, screen } from '@testing-library/react';
import NoticeList from './NoticeList';

const React = require('react');

describe('NoticeList keyboard actions', () => {
  it('marks a notice through Enter and exposes view more as a native button', () => {
    const onClick = jest.fn();
    const onViewMore = jest.fn();

    render(
      <NoticeList
        title="Notifications"
        tabKey="notification"
        list={[{ id: 'notice-1', key: 'notice-1', title: 'Deployment finished' }]}
        onClick={onClick}
        onViewMore={onViewMore}
        showClear={false}
        showViewMore
        viewMoreText="View more"
      />,
    );

    fireEvent.keyDown(screen.getByRole('button', { name: /Deployment finished/ }), {
      key: 'Enter',
    });
    fireEvent.click(screen.getByRole('button', { name: 'View more' }));

    expect(onClick).toHaveBeenCalledWith(expect.objectContaining({ id: 'notice-1' }));
    expect(onViewMore).toHaveBeenCalledTimes(1);
  });
});
