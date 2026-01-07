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

## VHS Tape Format (Demo Recording)

VHS (charmbracelet/vhs) is used for recording terminal demos. Run `vhs manual` for full docs.

### Available Commands

```tape
Output <path>.(gif|webm|mp4)   # Output file
Require <program>              # Require program exists
Set <setting> <value>          # Configure settings
Sleep <time>                   # Wait (e.g., 500ms, 2s)
Type "<string>"                # Type text
Enter [repeat]                 # Press enter
Ctrl [+Alt][+Shift]+<char>     # Control sequences
Backspace/Delete [repeat]      # Delete chars
Down/Up/Left/Right [repeat]    # Arrow keys
Tab [repeat]                   # Tab key
Escape                         # Escape key
Hide/Show                      # Hide/show recording
Wait[+Screen][@<timeout>] /<regexp>/  # Wait for pattern
Screenshot <path>.png          # Take screenshot
```

### Settings

```tape
Set Shell bash
Set FontSize 13
Set FontFamily "JetBrains Mono"
Set Width 1400
Set Height 700
Set Padding 15
Set Theme "Catppuccin Mocha"
Set TypingSpeed 40ms
Set CursorBlink false
Set Framerate 30
Set PlaybackSpeed 1.0
```

### String Quoting Rules

```tape
# Use backticks for commands with single quotes or special chars
Type `PS1='$ '`
Type `kubectl patch cm foo -p '{"data":{"key":"value"}}'`

# Use double quotes for regular commands
Type "clear"
Type "make build"

# Use single quotes for tmux style commands with double quotes inside
Type 'tmux setw pane-border-style "fg=#3a3a3a"'
```

### Tmux Pane Switching

```tape
# Method 1: Ctrl+B then arrow keys (RECOMMENDED)
Ctrl+B
Right
Sleep 500ms

# Method 2: Ctrl+B then Type o (toggle)
Ctrl+B
Type o
Sleep 500ms
```

### Hide/Show Blocks

```tape
Hide
# Setup commands here - not recorded
Type "make build >/dev/null 2>&1"
Enter
Sleep 2s
Show
# Recording starts here
```

### Recommended Tmux Demo Pattern

```tape
Hide
# Create detached session first
Type "tmux -f scripts/demo.conf new-session -d -s demo 'bash --norc --noprofile'"
Enter
Type "tmux split-window -h -t demo 'bash --norc --noprofile'"
Enter
# Set pane titles before attaching
Type "tmux select-pane -t demo:0.0 -T 'left pane'"
Enter
Type "tmux select-pane -t demo:0.1 -T 'right pane'"
Enter
Type "clear"
Enter
Show

# Attach and set prompts directly in panes
Type "tmux attach -t demo"
Enter
Sleep 1s
Type `PS1='$ ' && clear`
Enter
```

### Setting Pane Titles

Pane titles need to be set from WITHIN the pane using escape sequences (bash resets terminal title on start):

```tape
# Set terminal title using OSC 2 escape sequence (works with #{pane_title})
Type `PS1='$ ' && printf '\033]2;my title\033\\' && clear`
Enter
```

### What Doesn't Work

- Complex nested quotes: `Type "tmux send-keys \"PS1='$ '\""`
- Escaped double quotes inside double quotes
- `tmux send-keys` with commands containing both single and double quotes
- `tmux select-pane -T` before attaching (bash resets the title)
- Always prefer attaching to tmux and typing directly over `tmux send-keys`
