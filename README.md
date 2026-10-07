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

All configuration lives in a single `config.yml` file:

```yaml
host: 127.0.0.1
port: 1337
timeout: 10

actions:
  deploy:
    timeout: 60
    command: |
      echo "Deploying..."
      ./scripts/deploy.sh
    cron: "*/5 * * * *"
```

Actions are defined under the `actions` map, keyed by action id. Only `command` is required. `name`, `timeout`, `cwd`, `cron`, and `concurrency` are optional.

`concurrency` defaults to `1`. Set it to `0` to allow unlimited overlapping runs.

Restart the server after changing the config file.

The config file is validated on startup against a JSON Schema (`internal/config/config.schema.json`, embedded in the binary) and the server refuses to start on errors: unknown fields are rejected, every action requires a `command`, `cron` expressions must parse, action ids must not start with `@` (reserved for system actions), and `port` must be 1–65535.

## Reloading

`POST /api/reload` re-reads the config file without restarting:

1. The new config is validated — on error the server keeps serving the old config.
2. Only changes under `actions` take effect; anything else is reported in the response `warning` and needs a restart to take effect.
3. New runs are paused (triggers return `503`), in-flight runs drain, the config is swapped, and the scheduler re-syncs — unchanged entries keep their schedule.
4. Ticks missed during the pause get one catch-up run each against the new config.

The response reports what happened:

```json
{"status": "ok", "added": ["b"], "removed": [], "updated": ["a"], "backfilled": []}
```

Environment variables `RUNIC_HOST`, `RUNIC_PORT`, and `RUNIC_DATA_DIR` override config values. Set `RUNIC_CONFIG` or pass `--config <path>` to use a different config file.

> [!TIP]
> Always wrap your `cron` expression in quotes (e.g., `cron: "*/5 * * * *"`) to avoid YAML parsing errors, especially if the expression starts with `*`.

## Commands

```
runic serve [--config <path>]   Start the server (default config: config.yml)
runic version                   Show version info
```
