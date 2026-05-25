# runic

A modern crontab replacement with a web UI and log viewer. Define scheduled commands in YAML, track execution history, and browse logs — all from a single binary.

## Installation

```bash
curl -sL https://raw.githubusercontent.com/gera2ld/runic/main/install.sh | sh
```

This installs `runic` to `~/.local/bin/runic`. Add that directory to your PATH.

<details>
<summary>Manual install</summary>

1. Download the latest release from GitHub
2. Make it executable: `chmod +x runic`
3. Move to your PATH: `mv runic ~/.local/bin/`

</details>

## Configuration

Create a `config.yml` (optional, all fields have defaults):

```yaml
host: 127.0.0.1
port: 1337
timeout: 10
```

Environment variables `RUNIC_HOST`, `RUNIC_PORT`, and `RUNIC_DATA_DIR` override config values.

## Actions

Place YAML files in `actions/<id>.yml`:

```yaml
name: deploy
timeout: 60
command: |
  echo "Deploying..."
  ./scripts/deploy.sh
```

Only `command` is required. `name`, `timeout`, `cwd`, `cron`, and `concurrency` are optional.

`concurrency` defaults to `1`. Set it to `0` to allow unlimited overlapping runs.

> [!TIP]
> Always wrap your `cron` expression in quotes (e.g., `cron: "*/5 * * * *"`) to avoid YAML parsing errors, especially if the expression starts with `*`.

## Commands

```
runic           Start the server
runic update    Upgrade to the latest release
runic version   Show version info
```
