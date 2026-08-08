import { getIntl } from '@umijs/max';
import { Button, message, notification } from 'antd';
import React from 'react';
import defaultSettings from '../config/defaultSettings';

const { pwa } = defaultSettings;
const isHttps = document.location.protocol === 'https:';

export const clearCache = async () => {
  if (!window.caches) {
    return;
  }

  const keys = await window.caches.keys();
  await Promise.all(keys.map((key) => window.caches.delete(key)));
};

type WaitingServiceWorker = Pick<ServiceWorker, 'postMessage'>;

export const applyServiceWorkerUpdate = async (
  worker: WaitingServiceWorker | undefined,
  reloadPage: () => void = () => window.location.reload(),
) => {
  if (!worker) {
    return false;
  }

  await new Promise((resolve, reject) => {
    const channel = new MessageChannel();
    channel.port1.onmessage = (msgEvent) => {
      if (msgEvent.data.error) {
        reject(msgEvent.data.error);
      } else {
        resolve(msgEvent.data);
      }
    };
    worker.postMessage({ type: 'skip-waiting' }, [channel.port2]);
  });

  reloadPage();
  return true;
};

// if pwa is true
if (pwa) {
  // Notify user if offline now
  window.addEventListener('sw.offline', () => {
    message.warning(getIntl().formatMessage({ id: 'app.pwa.offline' }));
  });

  // Pop up a prompt on the page asking the user if they want to use the latest version
  window.addEventListener('sw.updated', (event: Event) => {
    const e = event as CustomEvent;
    const reloadSW = async () => {
      // Check if there is sw whose state is waiting in ServiceWorkerRegistration
      // https://developer.mozilla.org/en-US/docs/Web/API/ServiceWorkerRegistration
      const worker = e.detail && e.detail.waiting;
      return applyServiceWorkerUpdate(worker);
    };
    const key = `open${Date.now()}`;
    const btn = (
      <Button
        type="primary"
        onClick={() => {
          notification.destroy(key);
          void reloadSW().catch(() => {
            message.error(
              getIntl().formatMessage({
                id: 'app.pwa.serviceworker.updated.error',
                defaultMessage: 'Unable to apply the update. Please reload the page manually.',
              }),
            );
          });
        }}
      >
        {getIntl().formatMessage({ id: 'app.pwa.serviceworker.updated.ok' })}
      </Button>
    );
    notification.open({
      message: getIntl().formatMessage({ id: 'app.pwa.serviceworker.updated' }),
      description: getIntl().formatMessage({ id: 'app.pwa.serviceworker.updated.hint' }),
      btn,
      key,
      onClose: async () => null,
    });
  });
} else if ('serviceWorker' in navigator && isHttps) {
  // unregister service worker
  const { serviceWorker } = navigator;
  if (serviceWorker.getRegistrations) {
    serviceWorker.getRegistrations().then((sws) => {
      sws.forEach((sw) => {
        sw.unregister();
      });
    });
  }
  serviceWorker.getRegistration().then((sw) => {
    if (sw) sw.unregister();
  });

  void clearCache().catch(() => undefined);
}
