# ddm-agent
# DDM Agent

Thin MVP target-side service. For now it exposes health and a mock command
execution endpoint so the dataplane can later swap echo mode for agent-backed
execution.

## Run

```bash
PORT=8084 go run ./cmd/server
```

## Linux Install

Install the latest GitHub release as a systemd service:

```bash
curl -fsSL https://raw.githubusercontent.com/NUS-MTech-Capstone/ddm-agent/main/scripts/install-linux.sh | sudo sh
```

Optional overrides:

```bash
DDM_AGENT_VERSION=v0.1.0 \
DDM_AGENT_PORT=8084 \
sudo -E sh scripts/install-linux.sh
```

The installer writes:

```text
/usr/local/bin/ddm-agent
/etc/ddm-agent/ddm-agent.env
/etc/systemd/system/ddm-agent.service
```

Then it runs:

```bash
systemctl enable --now ddm-agent.service
```

## Endpoints

```http
GET  /healthz
POST /execute
```
