#!/bin/bash
# Initialize demo pane with title and clean prompt
# Usage: source demo-pane-init.sh "pane title"
TITLE="${1:-demo}"
echo -ne "\033]2;${TITLE}\033\\"
PS1='$ '
clear
