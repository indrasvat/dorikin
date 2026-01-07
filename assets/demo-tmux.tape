# Minimal tmux + TUI test
Output assets/demo-tmux.gif
Set Shell bash
Set FontSize 13
Set Width 1400
Set Height 700
Set Theme "Catppuccin Mocha"

Hide

# Set clean prompt for outer shell first
Type `PS1='$ '`
Enter
Sleep 200ms

# Kill any existing tmux session, create new one with clean bash shells (no rc files = no Starship)
Type "tmux kill-session -t demo 2>/dev/null"
Enter
Sleep 300ms

# Create session with default-command to use clean bash
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

# LEFT pane - set simple prompt and clear
Type `PS1='$ ' && clear`
Enter
Sleep 500ms

# Switch to RIGHT pane
Ctrl+b
Right
Sleep 500ms

# RIGHT pane - set simple prompt, clear, and launch TUI
Type `PS1='$ ' && clear`
Enter
Sleep 300ms

Type "~/GolandProjects/indrasvat-dorikin/bin/dorikin ui --refresh-interval 1 ~/GolandProjects/indrasvat-dorikin/testdata/track-manifests 2>/dev/null"
Enter
Sleep 5s

# Press 'o' to hide IN_SYNC
Type "o"
Sleep 2s

# Quit
Type "q"
Sleep 500ms

Hide
Type "tmux kill-session -t demo"
Enter
Show
