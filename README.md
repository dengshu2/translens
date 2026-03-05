# 🌐 TransLens

个人中英翻译学习工具 — 输入中文，获取地道美式英文表达，并自动保存翻译记录。

## ✨ 功能特性

- **中译英翻译** — 基于 [OpenRouter](https://openrouter.ai/) 接入 AI 模型，翻译为地道、口语化的美式英文
- **翻译历史** — 自动保存每条翻译记录，支持回顾与重新翻译
- **搜索与分页** — 支持关键词搜索（中/英），分页加载历史记录
- **删除记录** — 支持删除单条翻译记录
- **CSV 导出** — 一键导出全部历史记录为 CSV 文件，兼容 Excel
- **极简界面** — Notion 风格 UI，响应式设计，支持桌面端和移动端
- **快捷操作** — `Ctrl + Enter` 快捷翻译，一键复制结果
- **访问控制** — HTTP Basic Auth 保护，防止未授权访问
- **健康检查** — `/health` 端点支持 Docker 容器健康检测

## 🏗️ 技术栈

| 层级 | 技术 |
|---|---|
| 后端 | Go 1.24 + [Chi](https://github.com/go-chi/chi) Router |
| AI | [OpenRouter API](https://openrouter.ai/)（兼容 OpenAI 接口，支持多种模型） |
| 数据库 | SQLite ([modernc.org/sqlite](https://modernc.org/sqlite)，纯 Go 实现，无需 CGO) |
| 前端 | 原生 HTML/CSS/JS 单文件，Notion 风格设计 |
| 部署 | Docker + Docker Compose |

## 📁 项目结构

```
translens/
├── main.go              # 入口：配置加载、路由、服务启停
├── handler.go           # HTTP 处理器：翻译、历史、导出
├── openrouter.go        # OpenRouter API 客户端封装
├── db.go                # SQLite 数据库初始化与 CRUD
├── static/
│   └── index.html       # 前端单页应用
├── data/                # SQLite 数据库文件（自动创建）
├── Dockerfile           # 多阶段构建
├── docker-compose.yml   # 容器编排
├── .env.example         # 环境变量模板
└── .env                 # 本地环境变量（不提交到 Git）
```

## 🚀 快速开始

### 前置条件

- Go 1.24+（本地开发）
- Docker & Docker Compose（容器部署）
- [OpenRouter API Key](https://openrouter.ai/keys)

### 本地开发

```bash
# 1. 克隆项目
git clone <repo-url> && cd translens

# 2. 配置环境变量
cp .env.example .env
# 编辑 .env，填入你的 API Key

# 3. 启动
go run .
```

程序会自动加载 `.env` 文件（通过 [godotenv](https://github.com/joho/godotenv)），无需手动 `source`。

浏览器访问：http://localhost:8080

### Docker 部署

```bash
# 1. 配置环境变量
cp .env.example .env
# 编辑 .env，填入你的 API Key

# 2. 启动容器
docker compose up -d

# 3. 查看日志
docker compose logs -f
```

浏览器访问：http://localhost:7070

## ⚙️ 环境变量

| 变量 | 必填 | 默认值 | 说明 |
|---|---|---|---|
| `OPENROUTER_API_KEY` | ✅ | — | OpenRouter API 密钥 |
| `OPENROUTER_MODEL` | — | `google/gemini-2.5-flash-preview` | 模型名称（支持 OpenRouter 上所有模型） |
| `AUTH_USERNAME` | — | —（不设则跳过认证） | Basic Auth 用户名 |
| `AUTH_PASSWORD` | — | —（不设则跳过认证） | Basic Auth 密码 |
| `PORT` | — | `8080` | 服务监听端口 |
| `DB_PATH` | — | `./data/translations.db` | SQLite 数据库文件路径 |

> **Note:** `AUTH_USERNAME` 和 `AUTH_PASSWORD` 需同时设置才会启用认证。若未设置，所有请求将无需认证即可访问。

## 📡 API 接口

### `POST /api/translate`

翻译中文为英文。

```bash
curl -u user:pass -X POST http://localhost:8080/api/translate \
  -H "Content-Type: application/json" \
  -d '{"chinese": "今天天气真好"}'
```

```json
{
  "id": 1,
  "chinese": "今天天气真好",
  "english": "The weather's really nice today.",
  "created_at": "2026-03-03T11:30:00Z"
}
```

### `GET /api/history`

获取翻译历史（支持分页和搜索）。

| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `limit` | int | 20 | 每页条数（最大 100） |
| `offset` | int | 0 | 偏移量 |
| `q` | string | — | 搜索关键词（匹配中文或英文） |

```bash
curl -u user:pass 'http://localhost:8080/api/history?limit=10&offset=0&q=hello'
```

### `DELETE /api/translations/{id}`

删除指定 ID 的翻译记录。

```bash
curl -u user:pass -X DELETE http://localhost:8080/api/translations/1
```

### `GET /api/export/csv`

导出全部翻译记录为 CSV 文件。

```bash
curl -u user:pass -O http://localhost:8080/api/export/csv
```

### `GET /health`

健康检查端点（无需认证）。

```bash
curl http://localhost:8080/health
```

## 🐳 Docker 说明

- 容器内部固定监听 `8080` 端口
- 宿主机映射端口在 `docker-compose.yml` 的 `ports` 中配置（默认 `7070`）
- 数据库通过 volume `./data:/app/data` 持久化到宿主机
- 容器时区设置为 `Asia/Shanghai`

## 📄 License

MIT
