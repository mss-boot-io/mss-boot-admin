import { fireEvent, render, screen } from '@testing-library/react';
import NoticeIcon from './NoticeIcon';

const React = require('react');

jest.mock('../HeaderDropdown', () => {
  const ReactRuntime = require('react');

  return ({ children, dropdownRender, onOpenChange, open }: any) =>
    ReactRuntime.createElement(
      'div',
      null,
      ReactRuntime.cloneElement(children, {
        onClick: () => onOpenChange?.(!open),
      }),
      open ? dropdownRender?.() : null,
    );
});

const notices: API.Notice[] = [
  {
    id: 'notice-1',
    key: 'notice-1',
    title: 'Deployment complete',
    type: 'notification',
  },
];

const renderNoticeIcon = (onViewMore = jest.fn()) =>
  render(
    <NoticeIcon
      ariaLabel="Notifications"
      count={1}
      viewMoreText="View more"
      onViewMore={onViewMore}
    >
      <NoticeIcon.Tab
        tabKey="notification"
        title="Notifications"
        list={notices}
        showClear={false}
        showViewMore
      />
      <NoticeIcon.Tab
        tabKey="message"
        title="Messages"
        list={[]}
        showClear={false}
        showViewMore
      />
    </NoticeIcon>,
  );

describe('NoticeIcon popup focus', () => {
  beforeEach(() => {
    jest.spyOn(window, 'requestAnimationFrame').mockImplementation((callback) => {
      callback(0);
      return 1;
    });
    jest.spyOn(window, 'cancelAnimationFrame').mockImplementation(() => undefined);
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  it('cycles Tab focus inside the dialog and restores it after Escape', () => {
    renderNoticeIcon();
    const trigger = screen.getByRole('button', { name: 'Notifications' });
    const escapedKeydown = jest.fn();
    window.addEventListener('keydown', escapedKeydown);

    fireEvent.click(trigger);

    const dialog = screen.getByRole('dialog', { name: 'Notifications' });
    const activeTab = screen.getByRole('tab', { name: /Notifications/ });
    const messageTab = screen.getByRole('tab', { name: 'Messages' });
    const noticeItem = screen.getByRole('button', { name: /Deployment complete/ });
    const viewMore = screen.getByRole('button', { name: 'View more' });
    expect(document.activeElement).toBe(activeTab);

    fireEvent.keyDown(activeTab, { key: 'Tab' });
    expect(document.activeElement).toBe(messageTab);
    fireEvent.keyDown(messageTab, { key: 'Tab' });
    expect(document.activeElement).toBe(noticeItem);
    fireEvent.keyDown(noticeItem, { key: 'Tab' });
    expect(document.activeElement).toBe(viewMore);
    fireEvent.keyDown(viewMore, { key: 'Tab' });
    expect(document.activeElement).toBe(activeTab);
    fireEvent.keyDown(activeTab, { key: 'Tab', shiftKey: true });
    expect(document.activeElement).toBe(viewMore);
    expect(escapedKeydown).not.toHaveBeenCalled();

    fireEvent.keyDown(dialog, { key: 'Escape' });

    expect(screen.queryByRole('dialog', { name: 'Notifications' })).toBeNull();
    expect(document.activeElement).toBe(trigger);
    window.removeEventListener('keydown', escapedKeydown);
  });

  it('closes the popup and restores focus when navigating to view more', () => {
    const onViewMore = jest.fn();
    renderNoticeIcon(onViewMore);
    const trigger = screen.getByRole('button', { name: 'Notifications' });

    fireEvent.click(trigger);
    fireEvent.click(screen.getByRole('button', { name: 'View more' }));

    expect(onViewMore).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole('dialog', { name: 'Notifications' })).toBeNull();
    expect(document.activeElement).toBe(trigger);
  });
});
