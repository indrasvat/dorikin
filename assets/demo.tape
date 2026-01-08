# Dorikin Demo - Live Drift Detection

Output assets/demo.gif
Set Shell bash
Set FontSize 13
Set Width 1400
Set Height 700
Set Padding 15
Set Theme "Catppuccin Mocha"
Set TypingSpeed 40ms
Set CursorBlink false

Require tmux

# ══════════════════════════════════════════════════════════════════════════════
# Setup (hidden)
# ══════════════════════════════════════════════════════════════════════════════
Hide

# Build and reset test track
Type "make build >/dev/null 2>&1 && make track-reset >/dev/null 2>&1"
Enter
Sleep 3s

# Kill any existing demo session
Type "tmux kill-session -t demo 2>/dev/null"
Enter
Sleep 300ms

# Create tmux session with NO config (-f /dev/null ignores user config)
Type "tmux -f /dev/null new-session -d -s demo 'bash --norc --noprofile'"
Enter
Sleep 500ms

# Split pane
Type "tmux split-window -h -t demo 'bash --norc --noprofile'"
Enter
Sleep 500ms

# Apply dorikin styling
Type 'tmux set -t demo pane-border-style "fg=#3a3a3a"'
Enter
Sleep 100ms
Type 'tmux set -t demo pane-active-border-style "fg=#50c878"'
Enter
Sleep 100ms
Type "tmux set -t demo pane-border-status top"
Enter
Sleep 100ms
# Use conditional format: pane 1 (left) = "test track", pane 2 (right) = "dorikin tui"
# Note: pane-base-index is 1, so first pane is index 1, not 0
Type 'tmux set -t demo pane-border-format " #{?#{==:#{pane_index},1},test track,dorikin tui} "'
Enter
Sleep 100ms

# Status bar - dorikin branding
Type 'tmux set -t demo status-style "bg=#1a1a1a,fg=#e0e0e0"'
Enter
Sleep 100ms
Type `tmux set -t demo status-left '#[bg=#50c878,fg=#1a1a1a,bold] dorikin #[bg=#1a1a1a] '`
Enter
Sleep 100ms
Type `tmux set -t demo status-right '#[fg=#888888]drift detection demo #[bg=#f5a623,fg=#1a1a1a,bold] LIVE '`
Enter
Sleep 100ms
Type "tmux set -t demo status-left-length 20"
Enter
Sleep 100ms
Type "tmux set -t demo status-right-length 40"
Enter
Sleep 100ms
Type `tmux setw -t demo window-status-format ''`
Enter
Sleep 100ms
Type `tmux setw -t demo window-status-current-format ''`
Enter
Sleep 100ms

# Clear outer shell
Type "clear"
Enter
Sleep 200ms

Show

# Attach to tmux session (titles already set via send-keys)
Type "tmux attach -t demo"
Enter
Sleep 1s

# We start in RIGHT pane (pane 1) - launch TUI
Type "./bin/dorikin ui --refresh-interval 1 ./testdata/track-manifests 2>/dev/null"
Enter
Sleep 3s

# Enable auto-refresh
Type "a"
Sleep 2s

# ══════════════════════════════════════════════════════════════════════════════
# Demo: Inject drift from left pane
# ══════════════════════════════════════════════════════════════════════════════

# Switch to left pane
Ctrl+B
Left
Sleep 500ms

# Inject image drift (nginx:1.25.0 → 1.26.0)
Type "make track-drift <<< '2'"
Enter
Sleep 3s

# Inject configmap drift (TOKYO_DRIFT mode)
Type "make track-drift <<< '3'"
Enter
Sleep 3s

# ══════════════════════════════════════════════════════════════════════════════
# Demo: Show drift detection in TUI
# ══════════════════════════════════════════════════════════════════════════════

# Switch to right pane
Ctrl+B
Right
Sleep 2s

# Filter drifted only
Type "o"
Sleep 2s

# Navigate to drifted resource
Down
Sleep 500ms

# Show diff
Enter
Sleep 3s

# Go back
Escape
Sleep 2s

# ══════════════════════════════════════════════════════════════════════════════
# Demo: Show logs view
# ══════════════════════════════════════════════════════════════════════════════

# Open logs view
Type "L"
Sleep 2s

# Scroll down through logs
Down
Sleep 500ms
Down
Sleep 500ms
Down
Sleep 1s

# Cycle log filter (all → info+ → warn+)
Type "f"
Sleep 1s
Type "f"
Sleep 2s

# Return to list view
Escape
Sleep 1s

# Quit TUI
Type "q"
Sleep 500ms

# ══════════════════════════════════════════════════════════════════════════════
# Cleanup (hidden)
# ══════════════════════════════════════════════════════════════════════════════
Hide
Type "tmux kill-session -t demo"
Enter
Sleep 300ms
Show
