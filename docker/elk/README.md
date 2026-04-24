# Local ELK Stack

Kibana dashboards backed by your own OpenSmurfManager telemetry, running
entirely on your machine. Use this to iterate on queries and dashboards
while the hosted telemetry endpoint is still in the backlog.

## What it runs

- **Elasticsearch** 8.13.4 (single node, security off) — port 9200
- **Kibana** 8.13.4 — port 5601
- **Filebeat** 8.13.4 — tails the JSON-lines log files the desktop app
  writes and ships them to Elasticsearch as documents in the
  `osm-logs-YYYY.MM.DD` index

Requires Docker Desktop (or an equivalent) with file sharing enabled for
the drive containing your logs.

## Wire up your logs directory

The desktop client writes to `%APPDATA%\OpenSmurfManager\logs\` on
Windows. Point the stack at it via the `OSM_LOGS_DIR` env var.

**PowerShell:**

```powershell
$env:OSM_LOGS_DIR = "$env:APPDATA\OpenSmurfManager\logs"
docker compose -f docker/elk/docker-compose.yml up -d
```

**Git Bash / WSL:**

```bash
export OSM_LOGS_DIR="$APPDATA/OpenSmurfManager/logs"
docker compose -f docker/elk/docker-compose.yml up -d
```

If the env var is unset the compose file falls back to a placeholder dir
and Filebeat will simply find nothing.

## First view

1. Launch the desktop app once so it writes at least one `app.start`
   record.
2. Open http://localhost:5601
3. Stack Management → Data Views → create a view with index pattern
   `osm-logs-*` and timestamp field `@timestamp`.
4. Discover → the `body` field is the event name (`app.start`,
   `vault.unlock`, `account.add`, …). Resource fields live under
   `resource.*` (e.g. `resource.client.id` for DAU/MAU, `resource.os.type`
   for platform split).

## Useful starter queries

```
body: "app.start"
body: ("account.add" OR "account.edit" OR "account.delete")
attributes.success: false
```

DAU example (last 24h):

```
body: "app.start"
| stats cardinality(resource.client.id)
```

(expressed in Kibana's UI as a metric panel with a `Unique count` of
`resource.client.id`).

## Stopping / resetting

```bash
docker compose -f docker/elk/docker-compose.yml down          # stop
docker compose -f docker/elk/docker-compose.yml down -v       # stop + drop indexed data
```

## What this is **not**

- Not a production telemetry target. Filebeat can only tail files on the
  host it runs on — remote users' events stay on their machines until
  the client grows an HTTP shipper.
- No authentication / TLS on Elasticsearch. Do not expose these ports
  beyond localhost.
