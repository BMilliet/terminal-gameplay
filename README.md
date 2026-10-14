# Terminal Gameplay (tg)

A powerful terminal productivity tool that provides quick access to directories, environment variables, aliases, scripts, notes, and more through an intuitive TUI (Terminal User Interface).

<img src="./gif/tg_1.gif" alt="gif" width="750">

## Features

- 🚀 **GoTo**: Quickly navigate to your frequently used directories
- ⚡ **Scripts**: Create, edit, and execute Lua scripts with confirmation
- 📝 **Notes**: Keep quick notes and snippets at your fingertips
- 🔐 **Env**: Activate or deactivate environment variables in your current shell
- 🔗 **Alias**: Activate or deactivate command aliases in your current shell
- 🔎 **Fuzzy Search**: Find files and directories under the current directory or `$WORKSPACE` without an external `fzf` dependency
- 📁 **File Menu**: Inspect the current directory in an IDE-like, color-coded file tree

## Installation

### Prerequisites

- Go 1.25.6 or higher
- Git
- Neovim (`nvim`) for opening files from the file menu
- Lua (`lua`, `lua5.4`, `lua5.3`, or `luajit`) for script execution

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
    TG_SHELL_INTEGRATION=posix $HOME/.terminal-gameplay/terminal-gameplay
    local cmd_file="$HOME/.terminal-gameplay/cmd-exec"
    if [ -f "$cmd_file" ]; then
        local cmd=$(command cat "$cmd_file")
        command rm -f "$cmd_file"
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
    set -lx TG_SHELL_INTEGRATION fish
    $HOME/.terminal-gameplay/terminal-gameplay
    set -l cmd_file $HOME/.terminal-gameplay/cmd-exec
    if test -f $cmd_file
        set -l cmd (command cat $cmd_file)
        command rm -f $cmd_file
        eval $cmd
    end
end
```

After updating the Fish integration, reload it in existing sessions with
`source /path/to/terminal-gameplay/tg.fish`.

### Reload Your Shell

After adding the configuration:

```bash
# For Bash/Zsh
source ~/.bashrc  # or ~/.zshrc

# For Fish
source ~/.config/fish/config.fish
```

## Usage

Run `tg` to open the interactive interface:

```bash
tg
```

The quick menu provides access to saved directories, scripts, notes,
environment variables, aliases, settings, and search tools. The file menu
offers an editable file tree with previews and a Vim-inspired workflow for
staging filesystem changes before applying them.

Navigation works with the arrow keys or Vim-style keys. Use the prompts and
mode labels shown in the interface for the commands available in each context.
Filesystem and workspace searches can open files or change the current shell
directory through the configured shell integration.

### Configuration File

On first run, `tg` creates a configuration file at `~/.terminal-gameplay/config.json`:

```json
{
  "goTo": {
    "home": "~",
    "projects": "~/projects"
  },
  "scripts": {
    "update.lua": "Update local packages"
  },
  "notes": {
    "reminder": "Don't forget to commit your changes!"
  },
  "env": {
    "FOO": {
      "value": "123",
      "active": true
    }
  },
  "aliases": {
    "cat": {
      "value": "bat",
      "active": true
    }
  }
}
```

For manual configuration, the shorthand `"FOO": "123"` is also accepted and is
treated as active. The same shorthand is accepted for aliases.

Script descriptions live in `config.json`; the Lua files themselves are stored in `~/.terminal-gameplay/scripts`.

Keys starting with `div` can be used to group related items with visual
dividers:

```json
{
  "goTo": {
    "home": "~",
    "div": "⚙️ work projects",
    "frontend": "~/workspace/frontend-app",
    "backend": "~/workspace/backend-api",
    "div2": "🛠️ personal",
    "dotfiles": "~/dotfiles",
    "scripts": "~/scripts"
  }
}
```
