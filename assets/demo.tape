# Dorikin Demo - Live Drift Detection with tmux
# Shows TUI auto-refresh catching drift in real-time

Output assets/demo.gif
Set Shell bash
Set FontSize 13
Set Width 1400
Set Height 700
Set Padding 15
Set Theme "Catppuccin Mocha"
Set TypingSpeed 40ms
Set CursorBlink false

Hide

# Set clean prompt for outer shell
Type `PS1='$ '`
Enter
Sleep 200ms

# Kill any existing tmux session
Type "tmux kill-session -t demo 2>/dev/null"
Enter
Sleep 300ms

# Create session with clean bash shells (no rc files = no Starship)
Type "tmux new-session -d -s demo 'bash --norc --noprofile'"
Enter
Sleep 500ms

Type "tmux split-window -h -t demo 'bash --norc --noprofile'"
Enter
Sleep 500ms

Type "tmux set -t demo status off"
Enter
Sleep 300ms

# Clear outer shell before showing
Type "clear"
Enter
Sleep 200ms

Show

# Attach to tmux
Type "tmux attach -t demo"
Enter
Sleep 1s

# LEFT PANE: Set simple prompt
Type `PS1='cmd> ' && clear`
Enter
Sleep 500ms

# Switch to RIGHT pane
Ctrl+b
Right
Sleep 500ms

# RIGHT PANE: Set simple prompt and launch TUI
Type `PS1='tui> ' && clear`
Enter
Sleep 300ms

Type "./bin/dorikin ui --refresh-interval 1 ./testdata/track-manifests 2>/dev/null"
Enter
Sleep 3s

# Enable auto-refresh (toggle on with 'a')
Type "a"
Sleep 1s

# Show initial state - all resources IN_SYNC
Sleep 2s

# Switch to LEFT pane
Ctrl+b
Left
Sleep 500ms

# LEFT PANE: Show what we're about to do
Type "echo 'Injecting drift...'"
Enter
Sleep 1s

# Scale nginx replicas (drift!)
Type "kubectl --context colima-dorikin-ae86 -n dorikin-garage scale deploy/nginx --replicas=5"
Enter
Sleep 2s

# Modify configmap (more drift!)
Type `kubectl --context colima-dorikin-ae86 -n dorikin-garage patch cm racing-config --type merge -p '{"data":{"DRIFT_MODE":"TOKYO_DRIFT"}}'`
Enter
Sleep 2s

# Switch to RIGHT pane to see TUI caught the drift
Ctrl+b
Right
Sleep 3s

# Press 'o' to filter and show only drifted resources
Type "o"
Sleep 2s

# Navigate to first drifted resource
Down
Sleep 500ms

# Press Enter to see diff details
Enter
Sleep 3s

# Go back
Escape
Sleep 2s

# Quit TUI
Type "q"
Sleep 500ms

# Cleanup
Hide
Type "tmux kill-session -t demo"
Enter
Sleep 300ms
Show
