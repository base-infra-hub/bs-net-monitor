# BS-Net-Monitor

多租户 IP 网络连通性监控服务。按策略组对 IP 做周期性 ICMP Ping 探测，提供实时状态推送（WebSocket）、历史统计查询、Excel 导入导出，并内置一个 Vue Web 管理后台。部署形态为**单个 exe + 一个 config.yaml**。

## 技术栈

**后端**

- Go 1.25，Gin（HTTP 框架）
- GORM + PostgreSQL（需启用 TimescaleDB 扩展，ping 明细存超表）
- Redis（go-redis v9：Session 存储、Ticket 一次性凭证）
- pro-bing（ICMP Ping 探测）、gorilla/websocket（实时推送）
- excelize（Excel 导入导出）、yaml.v3（配置解析）

**前端**

- Vue 3 + TypeScript + Vite
- Naive UI + ECharts + axios + vue-router

## 项目架构

```
├── main.go                 # 入口：加载配置、初始化、内嵌 dist 提供 Web 后台
├── config.yaml             # 配置文件（部署时与 exe 放同一目录）
├── dist/                   # 前端构建产物（由 web/ 构建生成，编译时内嵌进 exe）
├── web/                    # 前端源码（Vue3 + Vite）
├── internal/
│   ├── conf/               # 配置加载：读取 exe 所在目录的 config.yaml
│   ├── app/                # 启动装配：DB 连接、自动建表、TimescaleDB 超表、Redis、检测引擎
│   ├── api/                # 路由注册
│   ├── handler/            # HTTP 层（auth / ip / tactics / tenant）
│   ├── service/            # 业务逻辑（含 Ticket 签发校验）
│   ├── repository/         # 数据访问层
│   ├── model/              # GORM 模型（IP / Tactics / PingResult）
│   ├── dto/                # 传输对象
│   ├── detector/           # 检测引擎：单例 Manager + 事件循环，每个 IP 一个 ping goroutine
│   └── constant/           # 常量（上下文 Key 定义等）
└── pkg/
    ├── auth/               # JWT（RS256 本地验签）+ Session（Redis）
    ├── middleware/         # CORS、双轨鉴权、租户注入
    ├── redis/              # Redis 客户端
    └── response/           # 统一响应格式
```

### 核心思路

- **检测引擎（detector）**：单例 Manager，所有状态变更（IP/策略增删改）封装为事件提交到事件循环串行处理，避免锁竞争；每个启用中的 IP 对应一个独立 ping goroutine，探测周期/超时/不稳定阈值由所属策略组（Tactics）驱动。探测结果写入 TimescaleDB 超表（历史），最新状态缓存在内存（实时统计与推送）。
- **双轨鉴权**：`WebAuthMiddleware` 优先按 **Bearer JWT** 校验（RS256 本地公钥验签，纯 CPU 无 I/O，快），其次按 **Session**（Cookie 或 Bearer 携带 sessionId，查 Redis）校验。Web 后台登录走 Session；三方系统接入走 JWT。部分接口（如租户列表）用 `SessionAuthMiddleware` 强制只允许 Session，与三方 JWT 隔离。
- **Ticket（WebSocket 鉴权）**：浏览器 WebSocket 无法自定义请求头，因此先通过 HTTP 接口申请一次性 Ticket（AES-GCM 加密 + Redis 存储、用后即焚、短有效期），再带 Ticket 建立 WebSocket 连接。
- **多租户**：多租户隔离完全由 **JWT 内置的 `tenant_id`** 决定。数据按租户安全隔离。

## 内置 Web 后台

- 前端源码在 `web/`，执行 `npm run build` 后由 Vite 输出到**项目根目录的 `dist/`**（`web/vite.config.ts` 中 `outDir: ../dist`，`base: /web/`）。
- 后端通过 `//go:embed all:dist`（main.go）把 `dist/` **整个编译进 exe**，运行时无需单独部署前端。
- 启动后访问 `http://<host>:<port>/web/` 进入管理后台（支持 history 路由回退到 index.html）。

## 编译

先构建前端，再编译后端（`go:embed` 要求编译时 `dist/` 必须存在）：

```bash
# 1. 构建前端 → 产物输出到项目根 dist/
cd web
npm install
npm run build
cd ..

# 2. 编译后端（-s -w 去掉符号表和调试信息，减小体积）
go build -ldflags="-s -w" -o bs-net-monitor.exe main.go
```

## 部署

**运行依赖**：PostgreSQL（启用 TimescaleDB 扩展）、Redis。

**部署目录**（只需两个文件）：

```
bs-net-monitor.exe
config.yaml          # 必须与 exe 同目录，启动时读取；缺失或不合法会直接退出
```

`config.yaml` 关键配置：

| 配置项 | 说明 |
| --- | --- |
| `server.http_port` | HTTP 监听端口，默认 8082 |
| `database.postgres.*` | PostgreSQL 连接信息 |
| `redis.*` | Redis 连接信息（Session / Ticket 存储） |
| `auth.admin.username / password` | Web 后台登录账号密码（默认 admin / admin123，生产环境务必修改） |
| `auth.rsa_public_key` | 三方接入 JWT 的 RSA 验签公钥，见下文「三方系统接入」 |
| `auth.service_tag` | JWT 中 `tag` 字段必须与此值完全一致才放行，防止其他服务的 JWT 越权访问。**必须填写，不得留空** |
| `auth.session_ttl_seconds` | Web 后台登录态有效期，默认 86400（24h） |
| `ticket.aes_key` | Ticket AES 密钥，长度必须 16 / 24 / 32 字节 |
| `ticket.expire_seconds` | Ticket 有效期，默认 60 秒 |
| `ws.statistics_push_interval_ms` | statistics（统计）推送间隔，毫秒，默认 1000 |
| `ws.realtime_push_interval_ms` | realtime（单 IP 明细）推送间隔，毫秒，默认 1000 |
| `log.path` | 日志文件路径，相对当前工作目录；留空只输出控制台，默认 `log/app.log` |
| `log.max_size_mb` | 单个日志文件最大大小（MB），默认 10 |
| `log.max_age_days` | 日志保留天数，默认 7 |
| `log.max_backups` | 保留旧日志文件数，默认 5 |
| `log.stdout` | 是否同时输出到控制台，默认 true |
| `log.compress` | 是否压缩旧日志，默认 false |

启动后自动完成：建表（`ips` / `tactics` / `ping_results`）、将 `ping_results` 转为 TimescaleDB 超表、启动检测引擎。

> 注意：config.yaml 含有数据库/Redis 密码等敏感信息，请妥善保管，不要提交到公共仓库。

> 时间规范：后端所有时间字段均以 **RFC3339 带时区**（如 `2026-07-18T12:00:00+08:00`）返回；浏览器端统一解析为本地时区展示，避免跨时区换算错误。

> 日志：默认写到当前工作目录的 `log/app.log`，按大小轮转并保留 7 天；可通过 `log.*` 配置项调整路径、保留策略和是否同时输出控制台。

## 三方系统接入（JWT RSA）

> 如果你不想自己实现 JWT 签发，可以直接拉取配套授权项目 [base-infra-hub/bs-auth](https://github.com/base-infra-hub/bs-auth)，配置好 RSA 密钥对与 `service_tag` 后即可签发本系统需要的 JWT。

业务接口对外以 **RS256 JWT** 鉴权。接入方需要：

### 1. 在 config.yaml 中声明公钥

把 `auth.rsa_public_key` 改成**与接入方签名私钥配对**的 RSA 公钥。支持两种格式：

- 完整 PEM（含 `-----BEGIN PUBLIC KEY-----` 头尾）
- 裸 Base64 字符串（直接粘贴公钥内容，程序自动包装为 PEM）

同时配置 `auth.service_tag`：JWT payload 中的 `tag` 字段必须与此值一致才放行；留空则不校验 tag。

### 2. 用配对私钥签发 RS256 JWT

JWT 要求：

- header：`alg` 必须为 `RS256`
- payload：
  - `tag`：必须等于 `auth.service_tag`（若配置了的话）
  - `tenant_id`：**必填。** 对应分配的租户空间标识（如 `tenantA`），用作多租户数据隔离
  - `exp`：可选，过期时间；支持 **Unix 时间戳（数字）** 或 **RFC3339/ISO 8601 字符串**（如 `2026-07-18T12:00:00+08:00`）
- 签名：使用与配置公钥配对的 RSA 私钥做 PKCS#1 v1.5 + SHA-256 签名

Node.js 签发示例：

```js
const jwt = require('jsonwebtoken')

const token = jwt.sign(
  { 
    tag: 'BS-Net-Monitor', 
    tenant_id: 'tenantA', // 👈 绑定租户空间
    exp: Math.floor(Date.now() / 1000) + 3600 
  },
  privateKeyPem,          // 与 config.yaml 中公钥配对的私钥（PEM）
  { algorithm: 'RS256' }
)
```

### 3. 携带凭证调用接口

- 请求头 `Authorization: Bearer <jwt>` (无须再传递 X-Tenant-Id 头部)

主要接口（均以 `/api/v1` 为前缀）：

| 接口 | 说明 |
| --- | --- |
| `GET/POST /ips`，`GET/POST/DELETE /ips/:ipId` | IP 管理 CRUD |
| `POST /ips/batch/update`，`POST /ips/batch/delete` | 批量启停 / 删除 |
| `POST /ips/import`，`GET /ips/export`，`GET /ips/template` | Excel 导入 / 导出 / 模板 |
| `GET /ips/history`，`GET /ips/history/detail` | 历史统计（按天聚合）/ 单分钟明细 |
| `GET/POST /tactics`，`GET/POST/DELETE /tactics/:tacticsId` | 策略组 CRUD |
| `GET /ips/live/statistics` | 当前租户在线 / 离线 / 不稳定统计 |
| `POST /ips/live/ticket` | 申请 WebSocket 一次性 Ticket |
| `GET /ips/live/subscribe?ticket=<ticket>` | WebSocket 实时状态推送（此接口凭 Ticket 鉴权，无需请求头） |

### WebSocket 推送设计

连接建立后，服务端有**两条独立的推送通道**，各自按配置间隔定时推送（默认均为 1 秒，见 `ws.*` 配置）：

**1. statistics —— 租户级统计推送**

向所有已连接客户端推送当前租户的在线/离线/不稳定总数，无需任何订阅动作：

```json
{ "type": "statistics", "data": { "online": 98, "offline": 2, "unstable": 1 } }
```

**2. realtime —— 单 IP 实时状态明细推送**

只向**已设置过滤器**的连接推送。客户端通过 WebSocket 发送过滤消息告诉服务端自己关心哪些 IP：

```json
// 订阅全部 IP（租户内）
{ "action": "realtimeFilter", "all": true }

// 按策略组订阅
{ "action": "realtimeFilter", "tacticsId": 123 }

// 按指定 IP 列表订阅
{ "action": "realtimeFilter", "ipIds": [1, 2, 3] }
```

- 未发送过 `realtimeFilter` 的连接不会收到 realtime 推送
- 三个条件同时给时优先级：`tacticsId` > `ipIds` > `all`
- 可随时重复发送，新的过滤器直接覆盖旧的（用于切换页面/分组）

realtime 推送的消息格式（时间统一为 RFC3339 带时区，前端按浏览器本地时区转换）：

```json
{
  "type": "realtime",
  "data": [
    { "ipId": 1, "tacticsId": 123, "latencyMs": 12, "status": 2, "time": "2026-07-18T12:00:00+08:00" }
  ]
}
```

`status` 取值：`0` 离线、`1` 不稳定、`2` 在线；`latencyMs` 为 `null` 表示本次探测超时。

> 时间规范：服务端所有时间字段均以 **RFC3339 带时区**（如 `2026-07-18T12:00:00+08:00`）返回；浏览器侧统一解析为本地时区展示，避免跨时区换算错误。

### WebSocket 接入流程

1. `POST /api/v1/ips/live/ticket`（仅带 JWT）→ 返回 `ticket` 与有效期
2. 立即建立连接：`ws://<host>:<port>/api/v1/ips/live/subscribe?ticket=<ticket>`
3. Ticket 一次性、短有效期（默认 60s），用后即焚；重连需重新申请
4. 连接后自动接收 statistics 推送；按需发送 `realtimeFilter` 开始接收 realtime 推送
