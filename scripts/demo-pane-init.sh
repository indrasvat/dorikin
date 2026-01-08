#!/bin/bash
# Initialize demo pane with title and clean prompt
# Usage: ./demo-pane-init.sh title_with_underscores
# Underscores are converted to spaces in the title
TITLE="${1//_/ }"
printf '\033]2;%s\033\\' "$TITLE"
PS1='$ '
clear
