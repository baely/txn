# Standardization Plan for TXN Project

## 1. Code Structure
- Move all domain logic to `internal/` directory
- Each domain should have consistent sub-package structure:
  - `models` - Data structures
  - `service` - Business logic
  - `api` - HTTP handlers
  - `storage` - Persistence layer

## 2. Error Handling
- Replace all `fmt.Println` error reporting with structured logging (slog)
- Reserve `panic` only for unrecoverable startup errors
- Return errors from functions rather than handling internally
- Use error wrapping for context

## 3. Logging
- Standardize on `slog` package for all logging
- Define standard log levels and usage
- Create helper for consistent logger initialization

## 4. Naming Conventions
- Use consistent receiver names across similar types
- Standardize on PascalCase for exported types/functions
- Standardize on camelCase for private variables/functions

## 5. Import Organization
- Standard library first
- External dependencies second
- Internal packages last
- Separate with blank lines

## 6. Documentation
- Add package documentation to all packages
- Document all exported functions, types, and variables
- Add inline comments for complex logic

## 7. HTTP Routing
- Standardize on Chi router
- Consistent middleware approach
- Consistent error response format

## Priority Tasks
1. Standardize logging and error handling
2. Normalize HTTP routing and middleware
3. Reorganize package structure
4. Apply consistent naming
5. Add documentation