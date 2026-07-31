import { test, expect } from '@playwright/test';

test.describe('WebSocket Online Status', () => {
  test('should return online status', async ({ page }) => {
    const response = await page.request.get('http://localhost:8080/admin/api/ws/online');
    const data = await response.json();

    expect(data).toHaveProperty('onlineConnections');
    expect(data).toHaveProperty('onlineUsers');
    expect(typeof data.onlineConnections).toBe('number');
    expect(typeof data.onlineUsers).toBe('number');
  });
});
