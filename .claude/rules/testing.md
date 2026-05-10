# Testing Rules

> Loaded by Claude Code automatically. Applies to all test-related work.

## File Organization

- Tests mirror source structure: `src/lib/auth.ts` → `tests/lib/auth.test.ts`
- Unit tests: `*.test.ts` — Integration tests: `*.spec.ts`
- Test helpers/fixtures: `tests/_helpers/`, `tests/_fixtures/`
- Factory functions: `tests/_factories/` (for generating test data)

## Writing Tests

- Each test file tests ONE module — don't mix concerns
- Use descriptive test names: `it("should return 404 when user not found")`
- Follow Arrange-Act-Assert (AAA) pattern in every test
- One assertion per test when possible — multiple only if testing same behavior
- Never test implementation details — test behavior and outputs

## What to Test

- **Always test:** business logic, API endpoints, data layer queries, error handling, edge cases
- **Skip testing:** framework internals, third-party libraries, simple getters/setters, types
- **Edge cases to cover:** empty input, null/undefined, boundary values, error paths

## Mocking Strategy

- Prefer real implementations over mocks when feasible
- Mock ONLY: external APIs, databases (for unit tests), time/date, environment variables
- For integration tests: use real database (test instance), real file system
- Never mock the module under test

## Test Data

- Use factory functions for generating test data — never hardcode in tests
- Each test creates its own data — never share mutable state between tests
- Clean up after integration tests — use beforeEach/afterEach or transaction rollback
- Never use production data in tests

## Coverage

- Minimum coverage for new code: [configured in CLAUDE.md]
- Coverage is a guide, not a goal — 100% coverage with bad tests is worse than 80% with good tests
- Focus coverage on business logic and error paths, not boilerplate
