# 🌐 TransLens

个人中英翻译学习工具 — 输入中文，获取地道美式英文表达，并自动保存翻译记录。

## ✨ 功能特性

- **中译英翻译** — 基于 [OpenRouter](https://openrouter.ai/) 接入 AI 模型，翻译为地道、口语化的美式英文
- **英文纠错** — 输入英文段落，AI 自动纠正语法和拼写错误，逐词 diff 高亮显示修改
- **翻译/纠错历史** — 自动保存每条记录，支持回顾、搜索、分页加载
- **删除记录** — 支持删除单条翻译或纠错记录
- **CSV 导出** — 一键导出全部历史记录为 CSV 文件，兼容 Excel
- **极简界面** — Notion 风格 UI，响应式设计，明/暗双主题
- **快捷操作** — `Ctrl + Enter` 快捷翻译/纠错，一键复制结果
- **邮箱认证** — Email + 密码注册登录，JWT 鉴权，支持可配置的注册开关
- **健康检查** — `/health` 端点支持 Docker 容器健康检测

## 🏗️ 技术栈

| 层级 | 技术 |
|---|---|
| 后端 | Go 1.24 + [Chi](https://github.com/go-chi/chi) Router |
| 认证 | Email/Password + JWT（bcrypt 哈希） |
| AI | [OpenRouter API](https://openrouter.ai/)（兼容 OpenAI 接口，支持多种模型） |
| 数据库 | SQLite ([modernc.org/sqlite](https://modernc.org/sqlite)，纯 Go 实现，无需 CGO) |
| 前端 | 原生 HTML/CSS/JS，Notion 风格设计 |
| 部署 | Docker + Docker Compose |

## 📁 项目结构

```
translens/
├── main.go              # 入口：配置加载、路由、服务启停
├── auth.go              # 认证服务：注册、登录、JWT 签发/验证
├── handler.go           # HTTP 处理器：翻译、纠错、历史、导出
├── handler_auth.go      # 认证处理器：注册、登录、配置端点
├── middleware.go         # 中间件：CORS、JWT 鉴权
├── openrouter.go        # OpenRouter API 客户端封装
├── db.go                # SQLite 数据库初始化与 CRUD
├── static/
│   ├── index.html       # 主页面（翻译 + 纠错）
│   ├── login.html       # 登录/注册页面
│   ├── app.js           # 前端逻辑
│   └── style.css        # 样式（明/暗双主题）
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
# 编辑 .env，填入 API Key 和 JWT Secret

# 3. 启动
go run .
```

浏览器访问：http://localhost:8080/login — 首次使用请注册账号。

### Docker 部署

```bash
# 1. 配置环境变量
cp .env.example .env
# 编辑 .env，填入 API Key 和 JWT Secret

# 2. 启动容器
docker compose up -d

# 3. 查看日志
docker compose logs -f
```

浏览器访问：http://localhost:7070/login

## ⚙️ 环境变量

| 变量 | 必填 | 默认值 | 说明 |
|---|---|---|---|
| `OPENROUTER_API_KEY` | 是 | — | OpenRouter API 密钥 |
| `OPENROUTER_MODEL` | — | `google/gemini-2.5-flash-preview` | 模型名称 |
| `JWT_SECRET` | 是 | — | JWT 签名密钥，建议 `openssl rand -hex 32` 生成 |
| `ENABLE_REGISTRATION` | — | `true` | 设为 `false` 关闭注册入口 |
| `PORT` | — | `8080` | 服务监听端口 |
| `DB_PATH` | — | `./data/translations.db` | SQLite 数据库文件路径 |

> 首次部署时保持 `ENABLE_REGISTRATION=true`，注册完管理员账号后可改为 `false` 并重启。

## 📡 API 接口

所有 `/api/*` 端点需要在请求头中携带 JWT：

```
Authorization: Bearer <token>
```

### 认证

#### `POST /auth/register`

注册新用户（需 `ENABLE_REGISTRATION=true`）。

```bash
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email": "you@example.com", "password": "your-password"}'
```

#### `POST /auth/login`

登录获取 JWT。

```bash
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "you@example.com", "password": "your-password"}'
```

#### `GET /api/config`

获取公开配置（无需认证）。

```json
{"registration_enabled": true}
```

### 翻译

#### `POST /api/translate`

```bash
curl -X POST http://localhost:8080/api/translate \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"chinese": "今天天气真好"}'
```

#### `GET /api/history?limit=20&offset=0&q=hello`

获取翻译历史（分页 + 搜索）。

#### `DELETE /api/translations/{id}`

删除指定翻译记录。

#### `GET /api/export/csv`

导出全部翻译记录为 CSV。

### 纠错

#### `POST /api/correct`

```bash
curl -X POST http://localhost:8080/api/correct \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"english": "I goes to the store yesterday."}'
```

#### `GET /api/corrections?limit=20&offset=0&q=went`

获取纠错历史。

#### `DELETE /api/corrections/{id}`

删除指定纠错记录。

#### `GET /api/export/corrections/csv`

导出全部纠错记录为 CSV。

### 健康检查

#### `GET /health`

无需认证，供 Docker/负载均衡器探测。

## 🐳 Docker 说明

- 容器内部固定监听 `8080` 端口
- 宿主机映射端口在 `docker-compose.yml` 的 `ports` 中配置（默认 `7070`）
- 数据库通过 volume `./data:/app/data` 持久化到宿主机
- 容器时区设置为 `Asia/Shanghai`

## 📄 License

MIT
