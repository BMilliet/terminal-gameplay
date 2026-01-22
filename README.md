# Terminal Gameplay (tg)

A powerful terminal productivity tool that provides quick access to directories (warp), custom commands, and notes through an intuitive TUI (Terminal User Interface).

## Features

- 🚀 **Warp**: Quickly navigate to your frequently used directories
- ⚡ **Commands**: Store and execute custom shell commands with ease
- 📝 **Notes**: Keep quick notes and snippets at your fingertips

## Installation

### Prerequisites

- Go 1.25.6 or higher
- Git

### Build from Source

1. Install dependencies:
```bash
make deps
```

2. Build the binary:
```bash
make build
```

This will build the binary and move it to `~/.terminal-gameplay/terminal-gameplay`.

## Configuration

### Shell Integration

The `tg` command needs to be integrated into your shell to work properly. This allows the tool to execute commands in your current shell context (e.g., changing directories).

#### For Bash/Zsh

Add to your `~/.bashrc` or `~/.zshrc`:

```bash
source /path/to/terminal-gameplay/tg.sh
```

Or copy the function directly:

```bash
tg() {
    $HOME/.terminal-gameplay/terminal-gameplay
    local cmd_file="$HOME/.terminal-gameplay/cmd-exec"
    if [ -f "$cmd_file" ]; then
        local cmd=$(cat "$cmd_file")
        rm -f "$cmd_file"
        eval "$cmd"
    fi
}
```

#### For Fish Shell

Add to your `~/.config/fish/config.fish`:

```fish
source /path/to/terminal-gameplay/tg.fish
```

Or copy the function directly:

```fish
function tg
    $HOME/.terminal-gameplay/terminal-gameplay
    set -l cmd_file $HOME/.terminal-gameplay/cmd-exec
    if test -f $cmd_file
        set -l cmd (cat $cmd_file)
        rm -f $cmd_file
        eval $cmd
    end
end
```

### Reload Your Shell

After adding the configuration:

```bash
# For Bash/Zsh
source ~/.bashrc  # or ~/.zshrc

# For Fish
source ~/.config/fish/config.fish
```

## Usage

Simply run:

```bash
tg
```

This will launch the interactive TUI where you can:

1. **Navigate** using arrow keys or `j`/`k`
2. **Select** the main section (Warp, Commands, or Notes)

### Configuration File

On first run, `tg` creates a configuration file at `~/.terminal-gameplay/config.json`:

```json
{
  "warp": {
    "home": "~",
    "projects": "~/projects"
  },
  "commands": {
    "update": "sudo apt update && sudo apt upgrade"
  },
  "notes": {
    "reminder": "Don't forget to commit your changes!"
  }
}
```

