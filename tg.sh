#!/bin/bash

# Terminal Gameplay (tg) - Bash/Zsh wrapper
# Add this to your ~/.bashrc or ~/.zshrc:
#
#   source /path/to/tg.sh

tg() {
    # Run the binary from ~/.terminal-gameplay/tg
    $HOME/.terminal-gameplay/terminal-gameplay
    
    # Check if command file exists
    local cmd_file="$HOME/.terminal-gameplay/cmd-exec"
    
    if [ -f "$cmd_file" ]; then
        # Read the command
        local cmd=$(cat "$cmd_file")
        
        # Delete the file immediately
        rm -f "$cmd_file"
        
        # Execute the command in current shell
        eval "$cmd"
    fi
}
