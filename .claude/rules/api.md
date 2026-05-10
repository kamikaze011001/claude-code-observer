# API Design Rules

> Loaded by Claude Code automatically. Applies to all API-related work.

## Response Format

All API responses follow a consistent structure:

```json
// Success
{ "data": { ... }, "meta": { "page": 1, "total": 100 } }

// Error
{ "error": { "code": "VALIDATION_ERROR", "message": "...", "details": [...] } }
```

- Never mix `data` and `error` in the same response
- Always include appropriate HTTP status codes
- Never leak stack traces or internal error details to clients

## Endpoint Conventions

- Use plural nouns: `/users`, `/products`, `/orders`
- Nest related resources: `/users/:id/orders`
- Use query params for filtering/sorting: `/products?category=shoes&sort=price`
- Use HTTP methods correctly: GET (read), POST (create), PUT (full update), PATCH (partial update), DELETE (remove)

## Validation

- Validate ALL input at the API boundary — never trust client data
- Use schema validation library ([Zod/Pydantic/Joi] — configured in CLAUDE.md)
- Return 400 with specific error details for validation failures
- Validate path params, query params, headers, and body

## Authentication & Authorization

- Always check authentication before processing any request
- Always check authorization (permissions) after authentication
- Return 401 for unauthenticated, 403 for unauthorized
- Never include sensitive data in URLs or query strings

## Pagination

- Default page size: [configured per project]
- Maximum page size: [configured per project]
- Return pagination metadata in response: `{ meta: { page, perPage, total, totalPages } }`
- Use cursor-based pagination for real-time data or large datasets

## Versioning

- API version in URL path: `/api/v1/...`
- Major version bumps only for breaking changes
- Document breaking changes in ADR
