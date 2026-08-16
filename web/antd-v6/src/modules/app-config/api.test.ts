import { describe, expect, it, vi } from 'vitest';
import { createAppConfigAPI } from './api';

describe('application config API', () => {
  it('uses the authorized group contract and exact nested data envelope', async () => {
    const client = vi.fn().mockResolvedValue({});
    const api = createAppConfigAPI(client);
    await api.saveGroup('storage', { maxSize: 10_485_760, allowedTypes: 'image/*' });
    expect(client).toHaveBeenCalledWith('/app-configs/storage', {
      method: 'PUT',
      data: { data: { maxSize: 10_485_760, allowedTypes: 'image/*' } },
      skipErrorHandler: true,
    });
  });

  it('parses the structured storage upload response', async () => {
    const client = vi.fn().mockResolvedValue({
      url: 'https://cdn.example/logo.png',
      filename: 'logo.png',
      size: 12,
      mimeType: 'image/png',
    });
    await expect(createAppConfigAPI(client).uploadLogo(new Blob(['logo']))).resolves.toBe(
      'https://cdn.example/logo.png',
    );
  });
});
