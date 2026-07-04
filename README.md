# Terminal Gameplay (tg)

A powerful terminal productivity tool that provides quick access to directories, environment variables, aliases, scripts, notes, and more through an intuitive TUI (Terminal User Interface).

<img src="./gif/tg_1.gif" alt="gif" width="750">

## Features

- 🚀 **GoTo**: Quickly navigate to your frequently used directories
- ⚡ **Scripts**: Create, edit, and execute Lua scripts with confirmation
- 📝 **Notes**: Keep quick notes and snippets at your fingertips
- 🔐 **Env**: Activate or deactivate environment variables in your current shell
- 🔗 **Alias**: Activate or deactivate command aliases in your current shell

## Installation

### Prerequisites

- Go 1.25.6 or higher
- Git
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

Simply run:

```bash
tg
```

This will launch the interactive TUI where you can:

1. **Navigate** using arrow keys or `j`/`k`
2. **Switch pages** using left/right arrows or `h`/`l`
3. **Search** by pressing `/` to activate fuzzy-find mode
4. **Select** an item by pressing Enter

In the `env` tab, press `a`, enter the key (such as `FOO`), then enter its value
(such as `123`). The list shows each saved value and its `active`/`inactive`
state. Press Enter on an env key to toggle that state. Active keys are exported
to the current shell and inherited by scripts; inactive keys are unset.

The `env` feature can be enabled or disabled under `settings` → `features`.
Disabling it hides the tab and unsets every managed key without deleting the
saved values or their active/inactive states.

The `alias` tab follows the same workflow. Press `a`, enter the alias word
(such as `cat`), then enter the command it should execute (such as `bat`).
The list shows each command and its state. Enter toggles the alias, and `dd`
removes it. The feature can be enabled or disabled under `settings` → `features`.

### Fuzzy Find Search

Press `/` to activate the fuzzy-find search mode. This feature allows you to quickly filter items by typing:

- **Type to search**: Characters you type will fuzzy-match against both item labels and values
- **Visual feedback**: Matching characters are highlighted in yellow with dark text for easy reading
- **Exit search**: Press `Esc` to close the search and return to normal navigation
- **Select from results**: Use arrow keys to navigate filtered results and Enter to select

The fuzzy-find searches through both the item title/label and its value, making it easy to find what you need quickly.

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

#### Visual Dividers

You can organize your lists with visual dividers to separate items into sections. Use keys starting with `div` (e.g., `div`, `div1`, `div2`, etc.) to create dividers:

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

**Features:**
- Dividers are displayed as subtle horizontal separators with italic text
- They cannot be selected - navigation automatically skips them
- Use any text after `div` key to identify different dividers (since JSON doesn't allow duplicate keys)
- Great for grouping related items visually
