# Laffey API

English | [中文](README_CN.md) | [日本語](README_JA.md)

Laffey API is a self-hosted subscription-to-API gateway. It turns upstream AI product subscriptions and account pools into managed API access for users, teams, and internal services.

This repository is maintained at [desalahy/Sub2api_laffey](https://github.com/desalahy/Sub2api_laffey). It is a Laffey-branded fork of the upstream Sub2API project, with local branding, frontend preview support, and Laffey-themed home/auth pages.

## What It Does

- Distributes upstream subscription quota through platform-managed API keys.
- Manages multiple upstream accounts and routing strategies.
- Tracks token usage, cost, quota, balance, and request statistics.
- Supports sticky sessions, concurrency limits, and rate limits.
- Provides user and admin dashboards for daily operation.
- Includes built-in payment integration for self-service top-up and subscription workflows.
- Supports frontend-only preview mode for reviewing public pages without a database.

## Recent Laffey Changes

- Laffey-themed home page with character artwork and gateway-focused visual elements.
- Laffey-themed login and registration shell.
- Localized default site subtitle for English, Chinese, and Japanese.
- Frontend preview mock service for `/home`, `/login`, `/register`, and dashboard route checks.

## Tech Stack

| Area | Stack |
| --- | --- |
| Backend | Go, Gin, Ent |
| Frontend | Vue 3, Vite, TailwindCSS, Pinia |
| Database | PostgreSQL |
| Cache/Queue | Redis |
| Deployment | Docker Compose, binary installer, systemd |

## Quick Start With Docker Compose

Docker Compose is the recommended deployment path for a full self-hosted instance.

```bash
git clone https://github.com/desalahy/Sub2api_laffey.git
cd Sub2api_laffey/deploy

cp .env.example .env
# Edit .env before starting. At minimum set POSTGRES_PASSWORD, JWT_SECRET,
# and TOTP_ENCRYPTION_KEY.

docker compose -f docker-compose.local.yml up -d
docker compose -f docker-compose.local.yml logs -f sub2api
```

Open the setup wizard:

```text
http://YOUR_SERVER_IP:8080
```

The local-directory compose file stores PostgreSQL, Redis, and application data under deploy-managed directories so backup and migration are easier.

## Binary Installer

The installer is useful when PostgreSQL and Redis already run on the server.

```bash
curl -sSL https://raw.githubusercontent.com/desalahy/Sub2api_laffey/master/deploy/install.sh | sudo bash
```

After installation:

```bash
sudo systemctl start sub2api
sudo systemctl enable sub2api
sudo journalctl -u sub2api -f
```

Open:

```text
http://YOUR_SERVER_IP:8080
```

## Build From Source

Prerequisites:

- Go toolchain compatible with `backend/go.mod`
- Node.js and pnpm
- PostgreSQL
- Redis

Build frontend:

```bash
cd frontend
pnpm install
pnpm run build
```

Build backend:

```bash
cd backend
go mod download
make build
```

Use `deploy/config.example.yaml` as the starting point for a production configuration.

## Frontend Preview Mode

Preview mode is for UI review only. It starts a mock API and a Vite frontend server, so public pages can be opened without PostgreSQL or Redis.

On Windows PowerShell:

```powershell
.\tools\start-frontend-preview.ps1
```

Default URLs:

```text
Frontend: http://localhost:3000/home
Mock API: http://127.0.0.1:18080/api/v1/settings/public
```

This mode is intended for checking `/home`, `/login`, `/register`, and route-level frontend behavior. It is not a production runtime.

## Development Checks

Frontend type check:

```bash
cd frontend
pnpm run typecheck
```

Frontend test suite:

```bash
cd frontend
pnpm run test:run
```

Backend tests:

```bash
cd backend
go test ./...
```

Repository-level shortcuts are also available through the root `Makefile`.

## Nginx Reverse Proxy Note

If Nginx is used in front of Laffey API, enable underscores in request headers:

```nginx
underscores_in_headers on;
```

Without this setting, headers such as `session_id` may be dropped, which can break sticky-session routing.

## Payment Documentation

Payment configuration is documented separately:

- [Payment configuration](docs/PAYMENT.md)
- [Payment configuration, Chinese](docs/PAYMENT_CN.md)

## Project Structure

```text
backend/      Go backend service
frontend/     Vue frontend application
deploy/       deployment templates, installer, Docker Compose files
docs/         feature documentation
tools/        local development and preview utilities
```

## Upstream Relationship

This project is a fork of Sub2API. Laffey API keeps compatibility with the upstream architecture while applying fork-specific branding, deployment defaults, UI changes, and release behavior.

Upstream project names, domains, and demo services are not official Laffey API services unless they are explicitly listed in this repository.

## License

This project follows the license included in [LICENSE](LICENSE).
