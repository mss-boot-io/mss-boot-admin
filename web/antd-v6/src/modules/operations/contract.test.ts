import { describe, expect, it } from 'vitest';
import {
  OperationsContractError,
  parseLoginLog,
  parseOperationsPage,
  parseRuntimeLogFiles,
  parseRuntimeLogPage,
  parseSystemConfigDetail,
  parseSystemConfigSummary,
  parseTaskDetail,
  parseTaskSummary,
  serializeSystemConfigWrite,
  serializeTaskWrite,
} from './contract';

const timestamp = '2026-08-16T06:00:00Z';

describe('operations contracts', () => {
  it('keeps task list projections free of executable payload fields', () => {
    const page = parseOperationsPage(
      {
        data: [
          {
            id: 'task-1',
            createdAt: timestamp,
            updatedAt: timestamp,
            name: 'Health check',
            provider: 'default',
            spec: '0 * * * * *',
            status: 'disabled',
            remark: 'safe summary',
            body: '{"password":"must-not-survive"}',
            metadata: '{"Authorization":"secret"}',
          },
        ],
        total: 1,
        current: 1,
        pageSize: 20,
      },
      { current: 1, pageSize: 20 },
      parseTaskSummary,
    );

    expect(page.data[0]).toEqual({
      id: 'task-1',
      createdAt: timestamp,
      updatedAt: timestamp,
      name: 'Health check',
      provider: 'default',
      spec: '0 * * * * *',
      status: 'disabled',
      checkedAt: undefined,
      remark: 'safe summary',
    });
    expect(JSON.stringify(page)).not.toContain('must-not-survive');
  });

  it('parses task detail arrays but serializes only writable fields', () => {
    const detail = parseTaskDetail({
      id: 'task-1',
      createdAt: timestamp,
      updatedAt: timestamp,
      name: 'Function task',
      provider: 'func',
      spec: '0 * * * * *',
      status: 'enabled',
      remark: '',
      command: '[]',
      args: '["one","two"]',
      method: 'test',
      timeout: 30,
    });
    expect(detail.args).toEqual(['one', 'two']);

    const serialized = serializeTaskWrite({
      name: ' Function task ',
      provider: 'func',
      spec: '0 * * * * *',
      method: 'test',
      args: ['one'],
    });
    expect(serialized).toEqual({
      name: 'Function task',
      provider: 'func',
      spec: '0 * * * * *',
      remark: '',
      timeout: 30,
      method: 'test',
      args: '["one"]',
    });
    expect(serialized).not.toHaveProperty('id');
    expect(serialized).not.toHaveProperty('status');
  });

  it('keeps opaque system configuration content out of summaries', () => {
    const summary = parseSystemConfigSummary({
      id: 'config-1',
      createdAt: timestamp,
      updatedAt: timestamp,
      name: 'private',
      ext: 'json',
      remark: '',
      isBuiltIn: true,
      content: '{"token":"secret"}',
    });
    expect(summary).not.toHaveProperty('content');

    const detail = parseSystemConfigDetail({
      ...summary,
      isBuiltIn: summary.builtIn,
      content: '{"enabled":true}',
    });
    expect(detail.content).toBe('{"enabled":true}');
    expect(serializeSystemConfigWrite(detail)).not.toHaveProperty('isBuiltIn');
  });

  it('rejects oversized pages, unsafe filenames, and oversized runtime rows', () => {
    expect(() =>
      parseOperationsPage(
        { data: Array.from({ length: 101 }, () => ({})), total: 101 },
        { current: 1, pageSize: 100 },
        parseTaskSummary,
      ),
    ).toThrow(OperationsContractError);
    expect(() => parseRuntimeLogFiles({ files: ['../admin.log'], truncated: false })).toThrow(
      OperationsContractError,
    );
    expect(() =>
      parseRuntimeLogPage({
        list: Array.from({ length: 101 }, () => ({
          timestamp: '',
          level: '',
          message: '',
          raw: '',
        })),
        total: 101,
      }),
    ).toThrow(OperationsContractError);
  });

  it('accepts legacy log rows whose status was not recorded', () => {
    expect(
      parseLoginLog({
        id: 'login-1',
        userID: 'user-1',
        username: 'operator',
        ip: '127.0.0.1',
        location: '',
        userAgent: '',
        status: '',
        message: '',
        loginAt: timestamp,
      }).status,
    ).toBe('');
  });
});
