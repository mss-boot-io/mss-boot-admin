import { test, expect } from '@playwright/test';

test.describe('Monitor API', () => {
  test('should return system monitor data', async ({ request }) => {
    const loginRes = await request.post('http://localhost:8080/admin/api/user/login', {
      data: { username: 'admin', password: '123456' },
    });
    const loginData = await loginRes.json();
    const token = loginData.token;

    const response = await request.get('http://localhost:8080/admin/api/monitor', {
      headers: { Authorization: `Bearer ${token}` },
    });

    expect(response.ok()).toBeTruthy();
    const data = await response.json();

    expect(data).toHaveProperty('cpuLogicalCore');
    expect(data).toHaveProperty('cpuPhysicalCore');
    expect(data).toHaveProperty('memoryTotal');
    expect(data).toHaveProperty('diskTotal');
    expect(data).toHaveProperty('goVersion');
    expect(data).toHaveProperty('uptime');
    expect(data).toHaveProperty('network');
    expect(data).toHaveProperty('runtime');
  });

  test('should have network stats', async ({ request }) => {
    const loginRes = await request.post('http://localhost:8080/admin/api/user/login', {
      data: { username: 'admin', password: '123456' },
    });
    const loginData = await loginRes.json();
    const token = loginData.token;

    const response = await request.get('http://localhost:8080/admin/api/monitor', {
      headers: { Authorization: `Bearer ${token}` },
    });
    const data = await response.json();

    expect(data.network).toBeDefined();
    expect(data.network).toHaveProperty('bytesSent');
    expect(data.network).toHaveProperty('bytesRecv');
    expect(data.network).toHaveProperty('connectionCount');
    expect(data.network.connectionCount).toHaveProperty('total');
    expect(data.network.connectionCount.total).toBeGreaterThanOrEqual(0);
  });

  test('should have runtime stats', async ({ request }) => {
    const loginRes = await request.post('http://localhost:8080/admin/api/user/login', {
      data: { username: 'admin', password: '123456' },
    });
    const loginData = await loginRes.json();
    const token = loginData.token;

    const response = await request.get('http://localhost:8080/admin/api/monitor', {
      headers: { Authorization: `Bearer ${token}` },
    });
    const data = await response.json();

    expect(data.runtime).toBeDefined();
    expect(data.runtime).toHaveProperty('goroutines');
    expect(data.runtime).toHaveProperty('heapAlloc');
    expect(data.runtime).toHaveProperty('numGC');
    expect(data.runtime.goroutines).toBeGreaterThan(0);
  });
});
