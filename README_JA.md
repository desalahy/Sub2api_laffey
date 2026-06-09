# Laffey API

[English](README.md) | [中文](README_CN.md) | 日本語

Laffey API は、セルフホスト可能なサブスクリプション API ゲートウェイです。上流の AI 製品サブスクリプション、アカウントプール、利用枠を、ユーザー、チーム、社内サービス向けの管理された API アクセスとして提供します。

本リポジトリは [desalahy/Sub2api_laffey](https://github.com/desalahy/Sub2api_laffey) で管理されています。上流の Sub2API をベースにした Laffey ブランドのフォークで、ローカルブランディング、フロントエンドプレビューモード、Laffey テーマのホーム/認証ページを含みます。

## 主な機能

- プラットフォームが発行する API Key で上流サブスクリプションの利用枠を配布。
- 複数の上流アカウントとルーティング戦略を管理。
- Token 使用量、コスト、クォータ、残高、リクエスト統計を記録。
- スティッキーセッション、同時実行数制限、レート制限に対応。
- ユーザー向けおよび管理者向けの Web ダッシュボードを提供。
- セルフサービスのチャージやサブスクリプション購入向けの決済連携を内蔵。
- データベースなしで公開ページを確認できるフロントエンド専用プレビューモードに対応。

## 最近の Laffey 変更

- ホームページに Laffey テーマのキャラクターアートと API ゲートウェイ向けの視覚要素を追加。
- ログイン/登録ページに Laffey テーマの外観を追加。
- 既定のサイトサブタイトルを英語、中国語、日本語でローカライズ。
- `/home`、`/login`、`/register`、ダッシュボードルート確認用のフロントエンドプレビュー mock サービスを追加。

## 技術スタック

| 領域 | スタック |
| --- | --- |
| Backend | Go, Gin, Ent |
| Frontend | Vue 3, Vite, TailwindCSS, Pinia |
| Database | PostgreSQL |
| Cache/Queue | Redis |
| Deployment | Docker Compose, binary installer, systemd |

## Docker Compose でのクイックスタート

フル構成のセルフホスト環境では Docker Compose の利用を推奨します。

```bash
git clone https://github.com/desalahy/Sub2api_laffey.git
cd Sub2api_laffey/deploy

cp .env.example .env
# 起動前に .env を編集してください。少なくとも POSTGRES_PASSWORD,
# JWT_SECRET, TOTP_ENCRYPTION_KEY を設定します。

docker compose -f docker-compose.local.yml up -d
docker compose -f docker-compose.local.yml logs -f sub2api
```

セットアップウィザードを開きます。

```text
http://YOUR_SERVER_IP:8080
```

`docker-compose.local.yml` は PostgreSQL、Redis、アプリケーションデータを deploy 配下のローカルディレクトリに保存するため、バックアップと移行が扱いやすくなります。

## バイナリインストーラー

サーバー上に PostgreSQL と Redis がすでにある場合は、インストーラースクリプトでサービスを配置できます。

```bash
curl -sSL https://raw.githubusercontent.com/desalahy/Sub2api_laffey/master/deploy/install.sh | sudo bash
```

インストール後によく使うコマンド:

```bash
sudo systemctl start sub2api
sudo systemctl enable sub2api
sudo journalctl -u sub2api -f
```

アクセス先:

```text
http://YOUR_SERVER_IP:8080
```

## ソースからビルド

必要なもの:

- `backend/go.mod` と互換性のある Go toolchain
- Node.js と pnpm
- PostgreSQL
- Redis

フロントエンド:

```bash
cd frontend
pnpm install
pnpm run build
```

バックエンド:

```bash
cd backend
go mod download
make build
```

ローカル開発や本番設定では、データベース、Redis、JWT、TOTP 暗号化キー、外部決済設定に対応する環境変数を用意してください。

## フロントエンドプレビューモード

Laffey テーマのホーム/認証ページは、PostgreSQL や Redis なしでもプレビューできます。Windows PowerShell 例:

```powershell
cd frontend
$env:VITE_PREVIEW_MODE="1"
pnpm run dev -- --host 0.0.0.0 --port 3000
```

確認ページ:

```text
http://localhost:3000/home
http://localhost:3000/login
http://localhost:3000/register
```

このモードはフロントエンド確認用です。実際のログイン、決済、API 利用にはバックエンドとデータベースが必要です。

## 開発時のチェック

フロントエンド:

```bash
cd frontend
pnpm run type-check
pnpm run build
```

バックエンド:

```bash
cd backend
go test ./...
```

## Nginx リバースプロキシの注意

Nginx の背後に配置する場合は、Cloudflare のようなプロキシ/CDN で提供される `CF-Connecting-IP` などの実クライアント IP ヘッダーをバックエンドへ渡してください。これによりレート制限、監査ログ、クライアント識別が正しく動作します。

例:

```nginx
proxy_set_header CF-Connecting-IP $http_cf_connecting_ip;
```

## 決済ドキュメント

決済設定の詳細は [docs/PAYMENT.md](docs/PAYMENT.md) を参照してください。

## プロジェクト構成

```text
backend/      Go API service
frontend/     Vue frontend
deploy/       Docker Compose, installer, service files
docs/         Project documentation
```

## 上流プロジェクトとの関係

Laffey API は Sub2API をベースにしたフォークです。本リポジトリ固有の変更は、Laffey ブランディング、公開ページのフロントエンド調整、ローカルプレビューフロー、デプロイ体験の整理に重点を置いています。上流由来の機能や設計は尊重しつつ、この README は本フォークの使い方とメンテナンス対象だけを説明します。

## ライセンス

このリポジトリのライセンスは、プロジェクト内のライセンスファイルおよび上流 Sub2API のライセンス条件に従います。
