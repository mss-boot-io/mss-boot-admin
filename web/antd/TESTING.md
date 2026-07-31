# Testing Guide

This guide provides comprehensive instructions for testing the `mss-boot-admin-antd` frontend application.

## Table of Contents

- [Test Types](#test-types)
- [Unit Tests](#unit-tests)
- [Integration Tests](#integration-tests)
- [End-to-End (E2E) Tests](#end-to-end-e2e-tests)
- [Mobile Testing](#mobile-testing)
- [Test Coverage](#test-coverage)
- [Development Workflow](#development-workflow)
- [Writing Tests](#writing-tests)
- [CI/CD Integration](#cicd-integration)

## Test Types

The project follows a multi-layered testing strategy:

| Test Type | Purpose | Tools | Coverage Requirement |
|-----------|---------|-------|---------------------|
| Unit Tests | Test individual functions, hooks, and components in isolation | Jest | ≥80% |
| Integration Tests | Test component interactions and API integration | React Testing Library + MSW | ≥80% |
| E2E Tests | Test complete user flows in real browsers | Playwright | Critical flows covered |

## Unit Tests

### Running Unit Tests

```bash
# Run all unit tests
pnpm test

# Run tests in watch mode
pnpm test --watch

# Run specific test file
pnpm test src/hooks/useUser.test.ts

# Generate coverage report
pnpm test --coverage
```

### Test Structure

Unit tests are located in:
- `src/hooks/__tests__/` - Custom hooks tests
- `src/components/__tests__/` - Reusable components tests  
- Individual component directories with `.test.tsx` files

### Example: Hook Test

```typescript
// src/hooks/__tests__/useUser.test.ts
import { renderHook, waitFor } from '@testing-library/react';
import { useUser } from '../useUser';

describe('useUser', () => {
  it('should fetch user successfully', async () => {
    const { result } = renderHook(() => useUser('user-1'));
    
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });
    
    expect(result.current.user).toBeDefined();
    expect(result.current.user?.id).toBe('user-1');
  });
  
  it('should handle error', async () => {
    // Mock API error
    const { result } = renderHook(() => useUser('invalid-id'));
    
    await waitFor(() => {
      expect(result.current.error).toBeTruthy();
    });
  });
});
```

### Example: Component Test

```typescript
// src/components/UserCard.test.tsx
import { render, screen } from '@testing-library/react';
import UserCard from './UserCard';

describe('UserCard', () => {
  it('renders user information correctly', () => {
    const mockUser = { id: '1', name: 'John Doe', email: 'john@example.com' };
    
    render(<UserCard user={mockUser} />);
    
    expect(screen.getByText('John Doe')).toBeInTheDocument();
    expect(screen.getByText('john@example.com')).toBeInTheDocument();
  });
  
  it('handles missing user gracefully', () => {
    render(<UserCard user={null} />);
    expect(screen.getByText('No user data')).toBeInTheDocument();
  });
});
```

## Integration Tests

### API Mocking with MSW

The project uses Mock Service Worker (MSW) for API mocking in integration tests.

```typescript
// src/mocks/handlers.ts
import { rest } from 'msw';

export const handlers = [
  rest.get('/admin/api/users/:id', (req, res, ctx) => {
    return res(
      ctx.json({
        id: req.params.id,
        name: 'Test User',
        email: 'test@example.com'
      })
    );
  }),
];
```

### Integration Test Setup

```typescript
// src/setupTests.ts
import { server } from './mocks/server';

beforeAll(() => server.listen());
afterEach(() => server.resetHandlers());
afterAll(() => server.close());
```

### Running Integration Tests

Integration tests are run as part of the unit test suite:

```bash
# Run all tests including integration tests
pnpm test --coverage

# Run specific integration test
pnpm test src/pages/User/List.test.tsx
```

## End-to-End (E2E) Tests

### Playwright Setup

E2E tests are located in the `e2e/` directory and use Playwright for browser automation.

### Running E2E Tests

```bash
# Install Playwright browsers (first time only)
pnpm run playwright:install

# Run all E2E tests
pnpm run test:e2e

# Run E2E tests in headed mode (visible browser)
pnpm run test:e2e -- --headed

# Run specific test file
pnpm run test:e2e -- e2e/login.spec.ts

# Run tests with specific browser
pnpm run test:e2e -- --project=chromium
pnpm run test:e2e -- --project=firefox
pnpm run test:e2e -- --project=webkit
```

### Example: Login E2E Test

```typescript
// e2e/login.spec.ts
import { test, expect } from '@playwright/test';

test('should login successfully', async ({ page }) => {
  await page.goto('/user/login');
  
  await page.fill('input[name="username"]', 'admin');
  await page.fill('input[name="password"]', '123456');
  await page.click('button[type="submit"]');
  
  await expect(page.locator('.ant-layout-header')).toBeVisible();
  await expect(page).toHaveURL('/');
});

test('should show error for invalid credentials', async ({ page }) => {
  await page.goto('/user/login');
  
  await page.fill('input[name="username"]', 'invalid');
  await page.fill('input[name="password"]', 'wrong');
  await page.click('button[type="submit"]');
  
  await expect(page.locator('.ant-alert-error')).toBeVisible();
});
```

## Mobile Testing

The project includes comprehensive mobile testing using Playwright's device emulation.

### Mobile Test Configuration

```typescript
// playwright.config.ts
import { devices } from '@playwright/test';

const config = {
  projects: [
    {
      name: 'iPhone 12 Pro',
      use: { ...devices['iPhone 12 Pro'] },
    },
    {
      name: 'Desktop Chrome',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
};
```

### Running Mobile Tests

```bash
# Run mobile-specific tests
pnpm run test:e2e -- --project='iPhone 12 Pro'

# Run all tests including mobile
pnpm run test:e2e
```

### Example: Mobile Navigation Test

```typescript
// e2e/mobile.spec.ts
import { test, expect } from '@playwright/test';

test('mobile menu should work correctly', async ({ page }) => {
  // Simulate mobile viewport
  await page.setViewportSize({ width: 390, height: 844 });
  
  await page.goto('/');
  
  // Mobile should show tab bar instead of sidebar
  await expect(page.locator('.mobile-tab-bar')).toBeVisible();
  await expect(page.locator('.ant-menu')).toBeHidden();
  
  // Test tab navigation
  await page.click('text=Users');
  await expect(page).toHaveURL('/user/list');
});
```

## Test Coverage

### Coverage Requirements

- **Overall Coverage**: ≥80%
- **New Code Coverage**: ≥80%
- **Critical Components**: ≥85%

### Generating Coverage Reports

```bash
# Generate HTML coverage report
pnpm test --coverage

# View coverage summary in terminal
pnpm test --coverage --collectCoverageFrom='src/**/*.{ts,tsx}'
```

### Coverage Configuration

The project uses Jest's built-in coverage reporting with the following configuration:

```javascript
// jest.config.js
module.exports = {
  collectCoverageFrom: [
    'src/**/*.{ts,tsx}',
    '!src/**/*.d.ts',
    '!src/main.tsx',
    '!src/generated/**',
  ],
  coverageThreshold: {
    global: {
      branches: 80,
      functions: 80,
      lines: 80,
      statements: 80,
    },
  },
};
```

## Development Workflow

Follow this workflow when developing new features:

```
1. DEVELOPMENT PHASE
   └── Write code
   └── Write tests (TDD preferred)
   └── Ensure compilation

2. TESTING PHASE (MANDATORY)
   ├── Unit Tests: pnpm test
   ├── Integration Tests: pnpm test
   └── E2E Tests: pnpm run test:e2e (for major features)

3. VERIFICATION PHASE
   ├── Check test coverage (≥80%)
   ├── All tests must pass
   └── Document test results
```

### Pre-commit Hooks

Before committing, ensure all tests pass:

```bash
# Run all tests
pnpm test --coverage

# Check coverage meets requirements
# Coverage summary must show ≥80%

# Run E2E tests for major changes
pnpm run test:e2e
```

## Writing Tests

### Best Practices

1. **Test Behavior, Not Implementation**
   - Focus on what the component does, not how it does it
   - Avoid testing internal state or implementation details

2. **Use Descriptive Test Names**
   ```typescript
   // Good
   it('shows error message when username is empty', () => { ... })
   
   // Bad
   it('handles validation', () => { ... })
   ```

3. **Keep Tests Independent**
   - Each test should be able to run independently
   - Use proper cleanup and setup

4. **Test Edge Cases**
   - Empty inputs
   - Error states
   - Loading states
   - Permission boundaries

### Test Structure Guidelines

| Component | Unit Tests | Integration Tests | E2E Tests | Min Coverage |
|-----------|-----------|-------------------|-----------|--------------|
| Hooks | ✅ Required | Optional | N/A | 80% |
| Components | ✅ Required | Optional | Optional | 75% |
| Utils | ✅ Required | Optional | N/A | 90% |
| Pages | ✅ Required | ✅ Required | Recommended | 80% |

## CI/CD Integration

### GitHub Actions

Tests are automatically run on every push and pull request via GitHub Actions.

```yaml
# .github/workflows/ci.yml
jobs:
  test:
    steps:
      - name: Run unit tests
        run: pnpm test -- --runInBand

      - name: Run E2E tests
        run: pnpm run test:e2e
```

### Pre-commit Hooks

The project uses Husky for pre-commit hooks:

```bash
# .husky/pre-commit
npx --no-install lint-staged
```

### Local Verification

```bash
pnpm run lint
pnpm tsc
pnpm test
pnpm run build:local
```

## Test Environment Setup

### Backend Dependencies

For E2E tests, you need a running backend:

```bash
# Start backend first
cd ../mss-boot-admin
go run . server

# Then run E2E tests
cd ../mss-boot-admin-antd
pnpm run test:e2e
```

### Environment Variables

Create a `.env.test` file for test-specific configurations:

```env
# .env.test
API_BASE_URL=http://localhost:8080/admin/api
MOCK_API=true
```

## Debugging Tests

### Unit Test Debugging

```bash
# Debug specific test
pnpm test --inspect-brk --runInBand src/hooks/useUser.test.ts
```

### E2E Test Debugging

```bash
# Open the Playwright inspector for step-by-step debugging
pnpm run test:e2e -- --debug
```

## Troubleshooting

### Common Issues

1. **Coverage Below Threshold**
   - Add missing test cases
   - Focus on untested branches
   - Use `/* istanbul ignore next */` sparingly

2. **E2E Tests Timing Out**
   - Increase timeout: `test.setTimeout(30000)`
   - Add proper waits: `await expect(locator).toBeVisible()`

3. **Mobile Tests Failing**
   - Ensure viewport is set correctly
   - Test both mobile and desktop layouts
   - Check responsive breakpoints

## Resources

- [Jest Documentation](https://jestjs.io/)
- [React Testing Library](https://testing-library.com/docs/react-testing-library/intro/)
- [Playwright Documentation](https://playwright.dev/)
- [MSW Documentation](https://mswjs.io/)

## License

MIT License - see [LICENSE](../LICENSE) for details.
