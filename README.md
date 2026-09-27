# 🔓 tf-unlock

A high-performance, zero-external-dependency Go CLI designed to inspect, diagnose, and safely break stale Terraform and OpenTofu remote state locks across AWS S3/DynamoDB, S3 Native object locks, Azure Blob Storage, and PostgreSQL backends.

---

## ⚡ Key Features

- 📦 **Zero Heavy Cloud SDK Invariant**: Eliminates multi-gigabyte cloud SDKs. Uses pure Go standard library (`net/http`, `crypto/hmac`, `crypto/sha256`, `encoding/json`, `database/sql`), `cobra v1.8.1`, and lightweight `github.com/lib/pq`.
- 🚀 **Ultra-Lightweight Binary**: Stripped binaries compile to under 10MB (target under 12MB).
- 🌐 **Universal Backend Support**:
  - 📦 **AWS S3 + DynamoDB**: Pure Go SigV4 signer with conditional `DeleteItem` and `attribute_exists(LockID)` checks.
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
go build -ldflags="-s -w" -o tf-unlock ./cmd/tf-unlock
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
├── cmd/
│   ├── tf-unlock/
│   │   └── main.go       # Root binary entrypoint
│   ├── auto.go           # CI blocker & stale lock auto-breaker
│   ├── break.go          # Controlled lock breaker with safety gates
│   ├── inspect.go        # Read-only lock inspector
│   └── root.go           # Cobra root command setup
├── pkg/
│   ├── backend/
│   │   ├── azure.go      # Azure Blob lease manager
│   │   ├── driver.go     # LockManager interface and LockInfo schema
│   │   ├── dynamodb.go   # AWS DynamoDB lock manager
│   │   ├── postgres.go   # PostgreSQL advisory lock manager
│   │   └── s3native.go   # S3 native object lock manager
│   ├── detector/
│   │   └── detector.go   # State file parser and backend detector
│   ├── safety/
│   │   └── safety.go     # Staleness check, lock ID verification, TTY prompt
│   ├── signer/
│   │   └── sigv4.go      # Pure Go AWS SigV4 signer
│   └── ui/
│       └── table.go      # Brutalist monospace terminal output
├── build.ps1             # PowerShell cross-compilation script
├── Makefile              # Make targets for build, test, and dist
└── go.mod                # Go module definition
```

---

## 🩺 Running Tests

Run the full test suite across all packages:

```bash
go test -v ./...
```

---

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

Copyright (c) 2026 x7ssss
