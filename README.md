# 🔓 tf-unlock

Safely break stale Terraform and OpenTofu state locks, but only after the runner that owns the lock is proven dead.

## 🚀 Quickstart

```yaml
- uses: x7ssss/tf-unlock@v1
  with: { backend-type: s3-dynamodb, table-name: terraform-locks, s3-bucket: my-bucket, s3-key: prod/terraform.tfstate }
```

Add `dry-run: true` to run every safety check without deleting anything. See [Action inputs](#-github-action-inputs).

## 🛡️ Safety Model: why naive `force-unlock` is dangerous

`terraform force-unlock <ID>` deletes the lock unconditionally. If the "stuck" run is actually still alive (slow API, long apply, network blip), a second `apply` starts against the same state: **split-brain**, with interleaved writes, corrupted state and orphaned resources.

tf-unlock only releases a lock when all of these checks pass:

1. **Tolerance window**: the lock must be older than `timeout-tolerance` (default 30m). Younger locks are refused.
2. **Lock ID match**: the lock currently held must be the `lock-id` you expect.
3. **Runner termination**: with `github-run-id`, the GitHub API must report that run as terminal (`completed`, `cancelled`, `timed_out`, `failure`). `queued`, `in_progress`, `waiting` or any unknown state, and any API error, block the unlock (fail closed).
4. **Atomic conditional delete**: DynamoDB uses `attribute_exists(LockID) AND contains(Info, :expectedID)`, S3 uses `If-Match: <etag>`. If another runner re-acquires the lock between check and delete, the delete fails instead of removing it.

`--dry-run` executes all four checks and then stops before the delete.

## 🧩 GitHub Action Inputs

| Input | Description | Default |
|---|---|---|
| `backend-type` | `s3-dynamodb` or `s3-native` (empty = auto-detect from `.terraform/terraform.tfstate`) | `""` |
| `table-name` | DynamoDB lock table | `""` |
| `lock-id` | Expected lock ID (empty = auto mode, only stale locks are cleared) | `""` |
| `s3-bucket` / `s3-key` | State bucket and key | `""` |
| `timeout-tolerance` | Minimum lock age before it may be broken | `30m` |
| `dry-run` | Run checks only, never delete | `false` |
| `github-token` | Token for workflow-run lookups | `${{ github.token }}` |
| `github-run-id` | Run that owns the lock; unlock only after it terminates | `""` |

AWS credentials are read from the standard `AWS_*` environment variables. The Action downloads the matching release binary (Linux/macOS/Windows, amd64/arm64) and falls back to building from source when Go is available.

---

## ⚡ Key Features

- 📦 **Zero Heavy Cloud SDK Invariant**: Eliminates multi-gigabyte cloud SDKs. Uses pure Go standard library (`net/http`, `crypto/hmac`, `crypto/sha256`, `encoding/json`, `database/sql`), `cobra v1.8.1`, and lightweight `github.com/lib/pq`.
- 🚀 **Ultra-Lightweight Binary**: Stripped binaries compile to under 10MB (target under 12MB).
- 🌐 **Universal Backend Support**:
  - 📦 **AWS S3 + DynamoDB**: Pure Go SigV4 signer with conditional `DeleteItem` using `attribute_exists(LockID) AND contains(Info, :expectedID)`.
  - 📦 **AWS S3 Native Object Locks**: Native S3 lockfiles (`<key>.tflock`) introduced in Terraform 1.10+ with `If-Match` conditional delete.
  - ☁️ **Azure Blob Storage**: Native Azure REST API lease breaking (`x-ms-lease-action: break`, `x-ms-lease-break-period: 0`) without modifying state blob content.
  - 🐘 **PostgreSQL Kernel Hang Buster**: Detects orphaned runner sessions in `pg_stat_activity` holding `advisory` locks and terminates them with `pg_terminate_backend(pid)`.
- 🔍 **Automatic State Detection**: Automatically parses `.terraform/terraform.tfstate` to identify the active backend type and configuration.
- 🛡️ **Safety Gates**:
  - ⏱️ Staleness verification (`--stale-after 30m`, default 30 minutes). Active locks younger than the threshold require `--force` to break.
  - 🛑 Double-check confirmation prompt in interactive TTY sessions.
  - 🎯 Target lock ID verification matching (`--lock-id`).
- 🚀 **CI/CD Native Mode (`auto`)**: Zero-config blocker for GitHub Actions and GitLab CI: exits 0 if clean or safely unlocked, exits 1 if an active pipeline holds the lock.
- 🖥️ **Monospace Brutalist UI**: Clear, tabular terminal visualization with status badges and timestamps.

---

## 📦 Installation

### 📦 Pre-built Binaries

Pre-compiled static binaries for Windows, Linux, and macOS are available in `dist/`:

- 📦 `dist/tf-unlock-linux-amd64` (~9.0 MB)
- 📦 `dist/tf-unlock-linux-arm64` (~8.4 MB)
- 📦 `dist/tf-unlock-windows-amd64.exe` (~9.3 MB)
- 📦 `dist/tf-unlock-windows-arm64.exe` (~8.5 MB)
- 📦 `dist/tf-unlock-darwin-amd64` (~9.3 MB)
- 📦 `dist/tf-unlock-darwin-arm64` (~8.7 MB)

### 🔧 Build From Source

Requires Go 1.23+ or Go 1.24+:

```bash
git clone https://github.com/x7ssss/tf-unlock.git
cd tf-unlock
go build -ldflags="-s -w" -o bin/tf-unlock .
```

Using build scripts:

```powershell
# PowerShell
.\build.ps1 -Test
.\build.ps1 -Dist
```

```bash
# Make
make test
make dist
```

---

## 🔧 CLI Usage

### 1. 🔍 Inspect Remote Lock (`inspect`)

Performs a read-only query to diagnose state locks:

```bash
tf-unlock inspect
tf-unlock inspect --state .terraform/terraform.tfstate --stale-after 30m
```

Example Output:
```text
+------------------------------------------------------------------------------+
|  TF-UNLOCK :: REMOTE STATE LOCK INSPECTOR
+------------------------------------------------------------------------------+
|  STATUS      : [LOCKED - STALE]
|  LOCK ID     : 3c9b782b-8a50-4822-8356-96a5bc3fe9b2
|  BACKEND     : s3-dynamodb
|  TARGET      : dynamodb://terraform-locks/prod/terraform.tfstate
|  OPERATION   : OperationTypePlan
|  WHO         : runner-42@ci-runner-pool
|  VERSION     : 1.9.5
|  CREATED     : 2026-09-27T08:00:00Z (2h 15m ago)
|  STALENESS   : STALE (age 2h 15m 0s exceeds 30m 0s threshold)
|  PATH        : prod/terraform.tfstate
|  INFO        : terraform plan executing
+------------------------------------------------------------------------------+
```

### 2. ⚡ Safely Break Lock (`break`)

Releases a state lock with safety checks:

```bash
# Standard release (prompts for confirmation and verifies lock is stale)
tf-unlock break

# Release specific lock ID
tf-unlock break --lock-id 3c9b782b-8a50-4822-8356-96a5bc3fe9b2

# Force break an active lock (bypasses staleness and confirmation)
tf-unlock break --force

# Custom staleness threshold
tf-unlock break --stale-after 45m
```

### 3. 🚀 CI Pipeline Blocker & Auto-Breaker (`auto`)

Designed as a pre-step in CI/CD pipelines before `terraform apply`:

```bash
tf-unlock auto --stale-after 1h
```

- ✅ **Exit code 0**: Backend is unlocked and clean, or a stale lock older than 1h was automatically released.
- ❌ **Exit code 1**: An active lock younger than 1h is currently held. CI aborts immediately to protect state.

#### 🐙 GitHub Actions Example

```yaml
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Setup Terraform
        uses: hashicorp/setup-terraform@v3

      - name: Init Terraform
        run: terraform init

      - name: Check and Clear Stale State Locks
        run: tf-unlock auto --stale-after 1h

      - name: Apply
        run: terraform apply -auto-approve
```

#### 🦊 GitLab CI Example

```yaml
terraform_deploy:
  stage: deploy
  script:
    - terraform init
    - tf-unlock auto --stale-after 1h
    - terraform apply -auto-approve
```

---

## 🌐 Supported Remote Backends

### 1. 📦 AWS S3 + DynamoDB (`s3-dynamodb`)
- 🔍 Partition key: `LockID` matching `<bucket>/<key>`.
- 🔐 Signed using pure Go AWS SigV4 (`pkg/signer/sigv4.go`).
- ⚡ Uses DynamoDB conditional delete expressions.
- 🛡️ Credentials resolved from `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`, or `~/.aws/credentials`.

### 2. 📦 AWS S3 Native Lockfiles (`s3-native`)
- 🔧 Compatible with Terraform 1.10+ native lockfiles (`use_lockfile = true`).
- 🔍 Inspects and deletes `<key>.tflock`.
- 🛡️ Employs `If-Match: <etag>` conditional HTTP delete to prevent race conditions.

### 3. ☁️ Azure Blob Storage (`azurerm`)
- 🔍 Inspects blob lease status (`x-ms-lease-status`, `x-ms-lease-state`).
- ⚡ Issues native HTTP PUT with `comp=lease`, `x-ms-lease-action: break`, and `x-ms-lease-break-period: 0`.
- 🛡️ Releases lease instantly without modifying blob contents.
- 🔐 Supports Azure SharedKey authentication, SAS tokens, and Bearer tokens.

### 4. 🐘 PostgreSQL Advisory Locks (`postgres`)
- 🔍 Queries `pg_locks` joined with `pg_stat_activity` for active `advisory` locks.
- ⚠️ Identifies runner processes that died without TCP FIN (kernel keepalive hangs).
- ⚡ Safely terminates stranded backend with `SELECT pg_terminate_backend(pid)`.

---

## 📁 Project Structure

```text
tf-unlock/
├── main.go               # Binary entrypoint
├── action.yml            # GitHub Marketplace composite action
├── action-entrypoint.sh  # Maps action inputs to CLI flags
├── cmd/                  # Cobra commands: auto, break, inspect, root
├── pkg/
│   ├── backend/          # DynamoDB, S3 native, Azure, PostgreSQL lock managers
│   ├── detector/         # State file parser and backend detector
│   ├── ghrun/            # GitHub workflow-run liveness checker
│   ├── safety/           # Tolerance, lock ID and runner-termination gates
│   ├── signer/           # Pure Go AWS SigV4 signer
│   └── ui/               # Terminal output
└── .github/workflows/ci.yml   # Tests and cross-platform release builds
```

---

## 🧪 Running Tests

```bash
go test -v -race -cover ./...
```

---

## 🤝 Community vs Team

| | **Community (Free, OSS / MIT)** | **Team ($29/mo or GitHub Sponsors)** |
|---|---|---|
| Local single-repo lock recovery | ✅ | ✅ |
| CLI (`inspect`, `break`, `auto`) | ✅ | ✅ |
| `--dry-run` mode | ✅ | ✅ |
| GitHub Action | ✅ | ✅ |
| Multi-repo global concurrency queue | - | ✅ |
| Slack / Teams incident alerts before unlocking | - | ✅ |
| Compliance audit logs | - | ✅ |

The Team features are not part of this repository. 👉 **[Get Team access via GitHub Sponsors](https://github.com/sponsors/x7ssss)**

---

## 📄 License


This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

Copyright (c) 2026 x7ssss
