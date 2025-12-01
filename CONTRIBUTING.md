# Contributing to Mistokenly

Thank you for your interest in contributing!

## Development Setup

1. Clone the repo and install dependencies
2. For Go services: `cd server && go mod download`
3. For examples: `cd examples && npm install`

## Building

```bash
cd server
go build ./cmd/...
```

## Code Style

- Go: Follow [Effective Go](https://golang.org/doc/effective_go)
- Run `gofmt` and `go vet` before submitting PRs
- Keep commits atomic and write clear messages

## Security

Report vulnerabilities in `SECURITY.md` – do NOT open public issues.

## Pull Requests

- Reference related issues
- Include tests for new features
- Update documentation if needed