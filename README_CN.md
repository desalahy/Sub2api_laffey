# Laffey API

[English](README.md) | 中文 | [日本語](README_JA.md)

Laffey API 是一个可自托管的订阅转 API 中转平台。它可以把上游 AI 产品订阅、账号池和额度统一封装成可管理的 API 访问能力，面向用户、团队或内部服务分发。

本仓库维护地址为 [desalahy/Sub2api_laffey](https://github.com/desalahy/Sub2api_laffey)。它是上游 Sub2API 项目的 Laffey 分支，包含 Laffey 品牌化、前端预览模式，以及面向首页、登录和注册页面的 Laffey 主题界面。

## 项目能力

- 通过平台生成的 API Key 分发上游订阅额度。
- 管理多个上游账号，并支持路由和调度策略。
- 统计 Token 用量、成本、额度、余额和请求数据。
- 支持会话保持、并发控制和速率限制。
- 提供用户端和管理员端 Web 控制台。
- 内置支付能力，支持自助充值和订阅购买流程。
- 支持无数据库前端预览，用于快速查看公开页面。

## 近期 Laffey 分支改动

- 首页增加 Laffey 主题主视觉，同时保留 API 中转站的产品表达。
- 登录和注册页面加入 Laffey 主题外观。
- 默认站点副标题支持中文、英文和日文切换。
- 新增前端预览 mock 服务，可在没有 PostgreSQL 和 Redis 的情况下查看 `/home`、`/login`、`/register` 等页面。

## 技术栈

| 模块 | 技术 |
| --- | --- |
| 后端 | Go, Gin, Ent |
| 前端 | Vue 3, Vite, TailwindCSS, Pinia |
| 数据库 | PostgreSQL |
| 缓存/队列 | Redis |
| 部署 | Docker Compose, 二进制安装器, systemd |

## Docker Compose 快速部署

推荐使用 Docker Compose 部署完整自托管实例。

```bash
git clone https://github.com/desalahy/Sub2api_laffey.git
cd Sub2api_laffey/deploy

cp .env.example .env
# 启动前请编辑 .env，至少设置 POSTGRES_PASSWORD、JWT_SECRET、
# TOTP_ENCRYPTION_KEY。

docker compose -f docker-compose.local.yml up -d
docker compose -f docker-compose.local.yml logs -f sub2api
```

打开初始化向导：

```text
http://你的服务器IP:8080
```

`docker-compose.local.yml` 使用本地目录保存 PostgreSQL、Redis 和应用数据，便于备份和迁移。

## 二进制安装器

如果服务器已经有 PostgreSQL 和 Redis，可以使用安装脚本部署服务。

```bash
curl -sSL https://raw.githubusercontent.com/desalahy/Sub2api_laffey/master/deploy/install.sh | sudo bash
```

安装后常用命令：

```bash
sudo systemctl start sub2api
sudo systemctl enable sub2api
sudo journalctl -u sub2api -f
```

打开：

```text
http://你的服务器IP:8080
```

## 源码构建

前置条件：

- 与 `backend/go.mod` 兼容的 Go 工具链
- Node.js 和 pnpm
- PostgreSQL
- Redis

构建前端：

```bash
cd frontend
pnpm install
pnpm run build
```

构建后端：

```bash
cd backend
go mod download
make build
```

生产配置可以从 `deploy/config.example.yaml` 开始调整。

## 前端预览模式

预览模式仅用于 UI 检查。它会启动一个 mock API 和 Vite 前端服务，因此不需要 PostgreSQL 或 Redis。

Windows PowerShell：

```powershell
.\tools\start-frontend-preview.ps1
```

默认地址：

```text
前端: http://localhost:3000/home
Mock API: http://127.0.0.1:18080/api/v1/settings/public
```

该模式适合检查 `/home`、`/login`、`/register` 和基础路由行为，不是生产运行方式。

## 开发检查

前端类型检查：

```bash
cd frontend
pnpm run typecheck
```

前端测试：

```bash
cd frontend
pnpm run test:run
```

后端测试：

```bash
cd backend
go test ./...
```

根目录 `Makefile` 也提供了部分常用检查和构建命令。

## Nginx 反向代理注意事项

如果使用 Nginx 反向代理 Laffey API，需要允许带下划线的请求头：

```nginx
underscores_in_headers on;
```

否则 `session_id` 等请求头可能被 Nginx 丢弃，导致会话保持路由异常。

## 支付文档

支付配置见独立文档：

- [支付配置，英文](docs/PAYMENT.md)
- [支付配置，中文](docs/PAYMENT_CN.md)

## 项目结构

```text
backend/      Go 后端服务
frontend/     Vue 前端应用
deploy/       部署模板、安装脚本和 Docker Compose 文件
docs/         功能文档
tools/        本地开发和预览工具
```

## 与上游项目的关系

本项目是 Sub2API 的分支。Laffey API 保持与上游架构兼容，同时加入分支自己的品牌、部署默认值、UI 改动和发布行为。

上游项目名称、域名和演示站点不代表 Laffey API 官方服务。以本仓库明确说明为准。

## 许可证

本项目遵循 [LICENSE](LICENSE) 中包含的许可证。
