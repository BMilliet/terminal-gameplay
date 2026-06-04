# Terminal Gameplay (tg) - Fish shell wrapper
# Add this to your ~/.config/fish/config.fish:
#
#   source /path/to/tg.fish

function tg
    set -lx TG_SHELL_INTEGRATION fish

    # Run the binary in the same directory
    "$HOME/.terminal-gameplay/terminal-gameplay"
    
    # Check if command file exists
    set -l cmd_file $HOME/.terminal-gameplay/cmd-exec
    
    if test -f $cmd_file
        # Read the command
        set -l cmd (command cat $cmd_file)
        
        # Delete the file immediately
        command rm -f $cmd_file
        
        # Execute the command in current shell
        eval $cmd
    end
end

# Apply managed env vars and aliases whenever this wrapper is sourced.
if test -x "$HOME/.terminal-gameplay/terminal-gameplay"
    eval (env TG_SHELL_INTEGRATION=fish "$HOME/.terminal-gameplay/terminal-gameplay" --shell-init)
end
