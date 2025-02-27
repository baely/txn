# TXN Project Guide

## Build Commands
- Run application: `go run main.go`
- Build binary: `go build -o txn`
- Run specific package tests: `go test ./internal/packagename`
- Run specific test: `go test ./internal/packagename -run TestName`

## Lint & Format
- Format code: `gofmt -w .`
- Lint code: `golangci-lint run`
- Check for race conditions: `go test -race ./...`

## Code Style
- **Imports**: Standard library first, then external packages, then internal packages
- **Formatting**: Follow `gofmt` conventions
- **Types**: Strong typing with interfaces where appropriate
- **Naming**: 
  - Use camelCase for variables and private functions
  - Use PascalCase for exported functions, types, and constants
- **Error Handling**: Check errors explicitly, no silent failures
- **Comments**: Document exported functions with godoc-style comments

## Structure
- Domain-driven organization with services in `internal/` directory
- Follow Go project standard layout practices