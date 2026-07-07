# DDM Agent

Target-side service for bounded DenDenMushi command execution. It accepts
requests only from callers holding a short-lived STS session token or
automation capability token, validates the token with STS, and executes the
command as the operating-system user running the agent.

The current protocol is line-oriented command execution, not a persistent PTY.
Shell state such as `cd` and environment changes is not preserved between
requests.

For Phase 2, this agent is an automation target for allowlisted diagnostics and
approved remediation. It is not the live terminal transport, does not preserve a
streaming shell, and must not execute model-generated free-form commands.

## Run

```bash
PORT=8084 \
STS_URL=http://localhost:8082 \
DDM_AGENT_HOST_ID=host-123 \
COMMAND_TIMEOUT=30s \
go run ./cmd/server
```

`DDM_AGENT_HOST_ID` is optional for local development. Set it in deployed
environments so an agent rejects requests targeting a different inventory host.

## Configuration

```text
PORT=8084
STS_URL=http://localhost:8082
STS_TIMEOUT=5s
COMMAND_TIMEOUT=30s
SHELL_PATH=/bin/sh
DDM_AGENT_HOST_ID=
MAX_COMMAND_BYTES=16384
MAX_OUTPUT_BYTES=1048576
```

`MAX_OUTPUT_BYTES` applies separately to stdout and stderr. Truncated output is
marked in the response.

## Design Patterns

DDM Agent uses an authorization Strategy before command execution.
`SessionAuthorizationStrategy` validates interactive owner requests, while
`AutomationAuthorizationStrategy` validates run-scoped automation capability
tokens and operation allowlists. Command execution remains behind the
`CommandRunner` interface so authorization and execution can be tested
independently.

## Release Distribution

`ddm-agent` is installed on target hosts, not deployed to the Kubernetes
cluster. The production distribution path is a GitHub Release containing static
Linux binaries for `amd64` and `arm64`; Docker/ECR image publishing is not the
supported deployment path for this service.

To publish a release, create and push a version tag:

```bash
git tag v0.1.0
git push origin v0.1.0
```

The release workflow runs Go CI, builds `ddm-agent-linux-amd64.tar.gz` and
`ddm-agent-linux-arm64.tar.gz`, generates `SHA256SUMS`, and attaches those
assets to the GitHub Release. The same workflow can be run manually for an
existing `v*` tag.

## Linux Install

Install the latest GitHub release as a systemd service:

```bash
curl -fsSL https://raw.githubusercontent.com/NUS-MTech-Capstone/ddm-agent/main/scripts/install-linux.sh | sudo sh
```

Hosts without `curl` can use `wget`:

```bash
wget -qO- https://raw.githubusercontent.com/NUS-MTech-Capstone/ddm-agent/main/scripts/install-linux.sh | sudo sh
```

Optional overrides:

```bash
DDM_AGENT_VERSION=v0.1.0 \
DDM_AGENT_PORT=8084 \
DDM_AGENT_STS_URL=https://sts.internal.example.com \
DDM_AGENT_HOST_ID=host-123 \
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

The installer creates a dedicated `ddm-agent` system user. Commands run as that
user, with Linux capabilities removed and `NoNewPrivileges=true`.

## Endpoints

```http
GET  /healthz
POST /execute
```

Business endpoints require `x-api-version: 1`. `GET /healthz` and
`GET /openapi.yaml` are exempt so probes and spec reads do not need custom
headers.

Example authenticated request:

```http
POST /execute
Authorization: Bearer <short-lived-session-token>
Content-Type: application/json
X-DDM-Session-ID: sess-123
X-DDM-User-ID: user-123
X-DDM-Host-ID: host-123
X-DDM-CR-ID: cr-123

{"command":"whoami"}
```

The agent calls STS `POST /tokens/validate` with the session ID, user ID, host,
change request, required `owner` mode, `ssh` connection type, and bearer token
before execution. Observer tokens cannot run commands. Tokens and command
contents are not logged.

Automation requests use an STS automation capability token and must include an
operation name that is present in the signed token's allowed operation list:

```http
POST /execute
Authorization: Bearer <short-lived-automation-token>
Content-Type: application/json
X-DDM-Run-ID: run-123
X-DDM-Incident-ID: inc-123
X-DDM-Host-ID: host-123

{"operation":"health_check","command":"systemctl is-active myapp"}
```

For automation requests, the agent calls `POST /tokens/automation/validate`,
checks the signed host and optional incident binding, and rejects operations not
listed in the token. The command still runs as the dedicated agent service
account; Cognito or workload identity is used for authorization and audit, not
as a Linux username.

Successful command execution, including a nonzero shell exit, returns:

```json
{
  "stdout": "",
  "stderr": "example error\n",
  "exitCode": 1
}
```

## API Specification

The OpenAPI contract is checked in at `api/openapi.yaml` and is served
unauthenticated at:

```http
GET /openapi.yaml
```

## Privileges

Do not run the agent as root by default. A remote command execution service
running as root turns any authentication or command-handling defect into full
host compromise.

Run it as a dedicated least-privileged account. If a small set of administrative
operations is required, expose tightly scoped root-owned helper programs or
specific `sudoers` entries rather than granting the whole agent unrestricted
root access.
