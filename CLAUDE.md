# Dorikin - Claude Code Instructions

## Project Overview

Dorikin is a Kubernetes configuration drift detector with a beautiful TUI, named after Keiichi Tsuchiya (the Drift King).

## Git Conventions

- **Create frequent, relevant, one-liner conventional commits**
- Use conventional commit format: `type: brief description`
  - `feat:` new features
  - `fix:` bug fixes
  - `refactor:` code refactoring
  - `chore:` maintenance tasks
  - `docs:` documentation changes
  - `test:` test additions/changes
  - `style:` formatting, styling changes
- Keep commits atomic and focused on a single change
- Examples:
  - `feat: add drift detector comparator`
  - `fix: handle nil pointer in k8s client`
  - `chore: update dependencies`

## Build & Development

- **Always use relevant Makefile targets**
- **Add new targets as you build the app and find missing ones**
- **Always prefer `make` targets as much as possible**

### Key Targets

```bash
make build       # Build the binary
make test        # Run tests
make lint        # Run linter (golangci-lint v2)
make fmt         # Format code
make ci          # Full CI pipeline
make run         # Run the TUI (development)
make run-scan    # Run a scan (development)
```

### Test Track (Local K8s Testing)

```bash
make track-setup   # Setup Colima k3s environment
make track-scan    # Run dorikin scan on test track
make track-drift   # Apply drift scenarios
make track-tui     # Launch TUI on test track
make track-reset   # Reset to baseline
make track-cleanup # Tear down environment
```

## Tech Stack

- **Go 1.25** - Using `sync.WaitGroup.Go()` method
- **Bubbletea v1.3.0** - TUI framework
- **Lipgloss v1.1.1** - Terminal styling
- **client-go v0.34.3** - Kubernetes API client
- **Cobra v1.9.1** - CLI framework
- **golangci-lint v2.7.2** - Linting

## Project Structure

```
cmd/dorikin/         # Entry point
internal/
  cli/               # CLI commands (Cobra)
  config/            # Configuration
  drift/             # Drift detection engine
  k8s/               # Kubernetes client
  loader/            # Manifest loading
  tui/               # TUI components (Bubbletea)
    components/      # Reusable TUI components
pkg/api/             # Public API types
testdata/            # Test manifests
scripts/             # Helper scripts (test-track.sh)
```

## Color Palette (AE86 Panda Trueno)

| Color | Hex | Usage |
|-------|-----|-------|
| Tsuchiya Jade | #00A86B | IN_SYNC, active selections |
| Hazard Yellow | #FFD700 | DRIFTED, warnings |
| Brake Red | #FF2D2D | MISSING, errors |
| JDM Purple | #9B59B6 | EXTRA resources |
| Neon Cyan | #00FFFF | Info, JSON paths |
| Panda Black | #0D0D0D | Background |
| Panda White | #F5F5F5 | Text |
