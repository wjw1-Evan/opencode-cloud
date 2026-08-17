# DevCapsule（开发胶囊舱）

基于 Docker 的编程开发环境容器管理平台：管理员批量生成账号、批量创建容器，用户登录后直接在浏览器中使用自己专属容器里的工具进行开发。平台内置 `opencode`、`vscode` 与 `jupyter` 三个系统模板开箱即用，同时管理员可以自由创建模板选择**任意提供 Web 界面的 Docker 镜像**（如 opencode、Dify、VS Code、JupyterLab…），自由配置端口、环境变量、启动命令与资源限额。每个用户对应一个独立容器，一一映射、互相隔离、代码持久化。

> 一句话：**管理员建一个分组 → 批量发账号 → 用户打开网页即可写代码 / 搭 AI 应用**。

## 特性一览

- **管理员端**
  - 批量生成用户名/密码：用户名前缀由课程名自动推导（如「Python 基础」→ `python001`），密码随机（默认 12 位，8 位起），已占用用户名自动跳过；可指定到期时间、CPU/内存限额
  - 一键「**生成账号并建容器**」：建号成功即自动为每个用户创建独立容器，浏览器自动下载 `accounts.csv`（含全部明文账号密码）供分发
  - 批量管理：同一张列表展示用户名/密码/课程/容器名/容器状态/端口，支持按课程筛选 → 勾选用户 → 批量**重建 / 重启 / 停止 / 删除**（幂等，失败可重试）
  - 镜像模板管理：镜像、主端口 + 附加端口、环境变量、启动命令、资源限额；系统模板内置不可删
  - 账号状态管理（`active / disabled / expired`）与到期自动停用；密码由用户自助修改（门户页「修改密码」），管理员可查看当前密码
  - 使用统计：总览页看用户/容器状态分布、在线数、近 24h 访问趋势、CPU/内存实时聚合、按课程分布
- **用户端**
  - 网页账号密码登录 → 自动进入自己专属的工具界面（管理员配置的镜像是什么就进什么工具）
  - 代码/数据持久化在专属 volume，容器重启不丢
- **平台能力**
  - 空闲自动停止（30 分钟无访问自动 `stop`，访问时秒级唤醒），账号到期自动停容器
  - 每容器 CPU / 内存 / PID 限额（`no-new-privileges` + 全部 capability 剔除），每容器独立随机密码（双层认证），SSE / WebSocket 全透传
  - 登录限流（每 IP 每分钟 10 次），`access_logs` 审计每个用户请求

## 适用场景

| 场景 | 说明 |
|---|---|
| **教学实验室** | 开课即建班：批量建号 → 批量建容器 → 发 CSV → 学员打开浏览器写代码 / 搭 AI 应用。按课程自由选择任意工具镜像模板，按总账统计，成本可控。 |
| **公司/企业** | 内部培训、实训营、招聘机考、研发环境分发。可通过镜像模板统一工具版本与权限，配合 TLS、审计日志、账号到期满足合规要求。 |

## 安装方法

> 前置要求：Linux 服务器（或装了 Docker Desktop 的开发机）、Docker Engine + Compose、git；生产部署另需 PostgreSQL（compose 中已内置）。

### 方式 A：docker-compose 一键部署（推荐）

仓库自带 `docker-compose.yml`（`api` + `nginx` + `postgres`），前端构建产物已 `embed` 进 Go 二进制，单文件分发。

```bash
# 1. 创建用户容器网络（api 与所有用户容器所在的专用网络，compose 声明为 external）
docker network create devcapsule_user-net

# 2. 准备密钥配置
cp .env.example .env
#    编辑 .env：JWT_SECRET（必改，可用 openssl rand -hex 32）、ADMIN_PASSWORD（必改）

# 3. 构建前端产物（嵌入 Go 二进制；若仓库已含 backend/internal/api/web/dist 可跳过）
cd frontend && npm ci && npm run build && cd ..

# 4. 启动（首次会自动构建 api 镜像、迁移建表、创建管理员、seed 系统模板）
docker compose up -d --build

# 5. 打开 http://<服务器>/ 用管理员账号登录
```

- 日志：`docker compose logs -f api`
- 升级：`git pull && cd frontend && npm run build && cd .. && docker compose up -d --build`
- 卸载：`docker compose down`（数据卷 `pgdata` 默认保留）

> ⚠️ api 容器需要读写宿主机的 `docker.sock`（compose 已挂载），请确保 Docker 可用。用户容器网络 `devcapsule_user-net` 必须**预先创建**，否则 compose 会启动失败。

### 方式 B：本地开发（前后端分离）

```bash
# 1. 数据库（本地跑单测可用 store 的内存实现，跑服务需要 PostgreSQL）
docker run -d --name devcapsule-db \
  -e POSTGRES_USER=opencode -e POSTGRES_PASSWORD=opencode \
  -e POSTGRES_DB=opencode -p 5432:5432 postgres:17-alpine

# 2. 后端（默认监听 :8080；启动时自动迁移建表、创建管理员、seed 系统模板、建网络）
export DATABASE_URL="postgres://opencode:opencode@localhost:5432/opencode?sslmode=disable"
export JWT_SECRET="dev-secret" ADMIN_PASSWORD="admin123" NETWORK_NAME="devcapsule_user-net"
cd backend && go run ./cmd/server

# 3. 前端（Vite dev server，仅代理 /api 与 /static；/platform 接口需自行在
#    vite.config.js 补充代理，或直接访问 :8080 上 embed 的完整平台页面）
cd frontend && npm install && npm run dev   # http://localhost:5173
```

> macOS（Docker Desktop）：用户容器间通过容器名路由，需先 `docker network create devcapsule_user-net`；Docker Desktop 的 VM 内存/CPU 需手动调高，一次跑 50 个容器本地吃力，生产建议 Linux 服务器。

### 首次使用（3 步建组）

1. 打开 `http://<服务器>/`，用管理员账号登录
2. **用户与容器** → 填课程（如「Python 基础」）、数量、选模板 → 点「生成账号并建容器」，浏览器自动下载 `accounts.csv`
3. **镜像模板** → 系统内置 opencode / vscode / jupyter 模板可直接用；也可创建自定义模板，填入任意工具镜像与端口

用户拿到账号后访问门户登录，随后访问站点根路径 `/` 即自动进入自己专属的工具界面。

> 生产环境强烈建议在 nginx 前加 TLS（见下文「生产部署」），用户仅通过 `https://<域名>/` 访问。

## 使用指南

### 管理员操作手册

| 步骤 | 操作 | 说明 |
|---|---|---|
| 1 | 建模板 | 填入任意工具镜像、主端口与附加端口、环境变量、启动命令、资源限额（系统内置 opencode / vscode / jupyter 模板） |
| 2 | 批量建号 | 输入课程、数量或显式用户名列表；用户名前缀由课程名自动推导（字母数字小写，如 `Python 基础`→`python001`，`2026 春季班`→`s2026001`），密码随机 12 位（默认），argon2 哈希入库，明文 `accounts.csv` 自动下载 |
| 3 | 批量建容器 | 「生成账号并建容器」自动完成，或选中用户 + 模板手动执行；后台并发：预拉镜像 → create（限额/网络/env/卷）→ start → 健康检查；已有容器自动跳过，可强制重建 |
| 4 | 日常管理 | 列表按课程筛选 → 勾选用户 → 批量重建（`containers/batch`，强制重建走 `force`）/重启/停止/删除（`users/batch/action`）；单容器启动走 `containers/{id}/start`；修改密码由用户自助完成，管理员可在列表查看当前密码 |
| 5 | 看统计 | Dashboard 看总量/活跃/在线、近 24h 访问趋势、CPU/内存实时聚合、课程分布；单个容器可看实时 docker stats |
| 6 | 到期回收 | 到期账号自动置为 `expired` 并停掉容器；空闲 30 分钟自动停，访问时秒级唤醒 |

### 用户使用手册

1. 用管理员发的账号密码登录门户
2. 系统自动进入自己的专属工具界面（无需任何本地安装；管理员建的是哪个模板就进哪个工具）
3. 在浏览器里使用对应工具：opencode 聊天写代码、code-server 在线 VS Code、Dify 可视化搭建 AI 应用等
4. 代码/数据保存在专属 volume（`code-{username}` 工作区 + `ocdata-{username}` 会话数据），关掉浏览器、容器重启都不丢
5. 一段时间不用容器会自动停止，再次访问时自动秒级唤醒
6. 可在门户页「修改密码」自助更换登录密码（新密码至少 8 位），管理员列表中会同步显示

## 环境变量与配置

| 变量 | 默认值 | 说明 |
|---|---|---|
| `ADDR` | `:8080` | 后端监听地址 |
| `DATABASE_URL` | `postgres://opencode:opencode@localhost:5432/opencode?sslmode=disable` | PostgreSQL 连接串 |
| `JWT_SECRET` | `dev-secret-change-me` | JWT 签名密钥，**生产必须修改** |
| `ADMIN_USERNAME` | `admin` | 自动创建的管理员用户名 |
| `ADMIN_PASSWORD` | `admin123` | 自动创建的管理员密码，**生产必须修改** |
| `NETWORK_NAME` | `devcapsule_user-net` | 用户容器所在自定义 bridge 网络 |
| `IDLE_TIMEOUT_MIN` | `30` | 空闲自动停止分钟数，`0` 关闭 |
| `BATCH_CONCURRENCY` | `5` | 批量建容器并发数 |
| `DOCKER_HOST` | `unix:///var/run/docker.sock` | Docker daemon 地址（compose 内已配置；api 容器内自动生效） |

compose 部署时把 `JWT_SECRET`、`ADMIN_PASSWORD` 等写入 `.env`（参考 `.env.example`）。

## 生产部署（公司场景）

### 1. TLS 与反代

仓库自带 `deploy/nginx.conf`（`nginx → api(:8080) → 用户容器`，监听 80，已配置 SSE/WS 透传）。生产在 nginx 上加 443 与证书：

```nginx
server {
    listen 443 ssl;
    server_name lab.example.com;
    ssl_certificate     /etc/nginx/tls/fullchain.pem;
    ssl_certificate_key /etc/nginx/tls/privkey.pem;

    client_max_body_size 20m;

    location / {
        proxy_pass http://api:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        # SSE 与 WebSocket 必需
        proxy_buffering off;
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

> ⚠️ **SSE 断流是最大坑**：opencode web 大量使用 SSE（`/global/event`）与 WebSocket。Go 反代已设 `FlushInterval: -1` 逐请求关闭缓冲，nginx 侧务必 `proxy_buffering off`，否则前端会崩溃。

### 2. 升级 / 上线检查清单

- [ ] `JWT_SECRET`、`ADMIN_PASSWORD` 已修改
- [ ] `devcapsule_user-net` 网络已创建（`docker network create devcapsule_user-net`）
- [ ] nginx 开启 TLS，HTTP 自动跳转 HTTPS
- [ ] opencode 镜像锁定版本（系统模板默认 `:latest`，建议改为固定 tag，`autoupdate: false`），升级前先在测试环境验证代理
- [ ] 资源限额已按分组规模设置；内存不足的机器开启空闲自动停
- [ ] 定期备份 PostgreSQL
- [ ] 防火墙仅放行 `443`（compose 默认映射 `80`），`8080` 与 docker 相关端口不对外

### 3. 备份与恢复

备份 `PostgreSQL`（用户、模板、容器绑定关系）即可恢复平台配置；用户代码在宿主机 volume 中，可按需快照：

```bash
pg_dump "$DATABASE_URL" > opencode-backup-$(date +%F).sql
```

### 4. 监控与告警

- 宿主机：关注 Docker daemon、磁盘（volume 增长）、内存水位
- 应用：`GET /api/health` 探活；`/platform/admin/stats/dashboard` 看在线与资源
- 审计：`access_logs` 表记录每个用户请求的时间、路径、状态、耗时

## 核心设计决策

| 决策项 | 结论 | 理由 |
|---|---|---|
| 容器访问方式 | HTTP 反向代理 | 不做宿主机端口映射，避免端口耗尽，可扩展性好 |
| 后端技术栈 | Go | 官方 Docker SDK、单二进制部署（前端 embed）、代理层性能好 |
| 用户路由 | 根路径 + JWT 身份识别 | 登录用户的根路径请求自动转发到其容器，工具无需感知前缀 |
| 认证 | 外层平台 JWT + 内层容器随机密码 | 双层防御，用户只感知一层登录 |
| 资源控制 | 每容器 CPU/内存/PID 限额 + 空闲自动停 | 控住 50 容器内存成本 |

## 系统架构

```
                    ┌──────────────── docker host ────────────────┐
                    │                                             │
 用户浏览器/管理员浏览器 │  docker-compose                            │
        │           │  ┌──────────────────────────────────────┐   │
        ▼           │  │ nginx :80 (TLS 可选)                  │   │
  :80 (nginx)       │  │  ┌────────────────────────────────┐  │   │
        │           │  │  │ api (Go 单二进制, 聚合三层)       │  │   │
        ▼           │  │  │  ├─ REST API 认证/用户/模板/统计  │  │   │
  ┌──────────┐       │  │  │  ├─ 反向代理 JWT→容器           │  │   │
  │ 管理端 UI │◀──────▶│  │  │  └─ Docker SDK 编排           │  │   │
  │ 用户门户  │       │  │  └───┬───────────────┬───────────┘  │   │
  └──────────┘       │  │      │ docker.sock   │ Postgres      │   │
        │            │  │ ┌────▼─────┐    ┌────▼─────┐         │   │
        └────────────│  │ │  Docker  │    │ postgres │         │   │
                     │  │ └──────────┘    └──────────┘         │   │
                     │  │ api 同时加入 user-net                  │   │
                     │  │       │                                │   │
                     │  │  ┌────▼──────────────── user-net ────┐ │   │
                     │  │  │  user-001  user-002  ... user-050  │ │   │
                     │  │  │  工具 Web 服务 :主端口 / 附加端口    │ │   │
                     │  │  │  volume: code-001  ocdata-001      │ │   │
                     │  │  └────────────────────────────────────┘ │   │
                     └────────────────────────────────────────────┘
```

**路由约定**：平台基于**根路径 + JWT 身份识别**路由，不依赖任何路径前缀。用户登录后访问站点根路径 `/`，代理解析其登录态（`Authorization: Bearer` 或 `access_token` cookie）确定所属容器，原样转发到 `user-{username}:{主端口}`——工具无需感知前缀，资源/API 路径天然兼容。未登录与管理员访问根路径获得平台 SPA（`/portal`、`/admin`、`/static/*` 恒为平台页面）。

**附加端口**：开发调试中可能需要同时访问多个服务端口（如开发工具一个端口、运行中的应用另一个端口）。模板可配置**一个主端口 + 多个附加端口**，附加端口走 `/port/{port}/`（如 `/port/3000/`），代理校验端口在模板白名单内后带双层认证转发到对应端口，所有端口共用同一容器、同一隔离与持久化。

## 核心流程

### 批量建号（管理员）

1. 输入课程/分组（如「Python 基础」）与数量（1–1000），或直接给显式用户名列表（`usernames`）
2. 用户名前缀由课程名推导（取字母数字小写，空则 `stu`，数字开头加 `s`），生成 `python001`~`python0NN` + 随机密码（默认 12 位，可配长度），已占用用户名自动跳过
3. 密码 argon2 哈希入库（明文存 `password_plain` 供管理员查看），响应直接返回明文账号，前端下载 `accounts.csv`
4. 可配置 `expires_in_days`（到期自动置 `expired` 并停容器）与 CPU/内存限额
5. 账号状态：`active / disabled / expired`，管理员可禁用/延长到期/改课程/重置密码

### 批量建容器（管理员，幂等）

1. 选模板（镜像、主端口、附加端口、环境变量、启动命令、资源限额）+ 勾选用户（或建号后自动建）
2. 后台 worker 池并发（`BATCH_CONCURRENCY`）执行：`预拉镜像 → create（限额/网络/环境/卷，label devcapsule=managed，restart=unless-stopped）→ start → 健康检查`（docker exec 在容器内探测 HTTP 端口，30s）
3. 已有容器默认跳过（可勾选强制重建），失败任务记录可重试
4. 建容器时注入 `OPENCODE_SERVER_USERNAME/PASSWORD/BASE_PATH`（每容器独立 24 位随机密码，双层认证）与 `OPENCODE_WORKDIR`；卷：`code-{username}`（工作区）+ `ocdata-{username}`（会话数据）
5. 用户与容器一一对应：删除用户即移除其容器；批量操作（重建/重启/停止/删除）直接作用于选中的用户及其容器

### 用户访问

1. 浏览器打开门户 → 账号密码登录（登录限流：每 IP 每分钟 10 次）
2. 访问站点根路径 `/` → 自动进入自己的工具（opencode / VS Code / Dify…）；运行中的应用走 `/port/{port}/` 附加端口路由；容器未运行时自动 `start` 并等待健康（15s）
3. 代码持久化在专属 volume，容器重启不丢；请求实时写入 `access_logs`（在线数与空闲判定依据）

### 后台周期任务（每 5 分钟）

`账号到期自动置 expired 并停容器 → 空闲超时自动 stop（IDLE_TIMEOUT_MIN）→ 数据库容器记录与 Docker 实况 reconcile（状态调和）`

## 应用模板与用户容器

平台内置三个**系统模板**（不可删除），其余工具通过模板机制自由接入——模板与具体工具**解耦**，任意提供 HTTP 端口的 Web 应用都可以下发为每个用户的专属容器。**opencode 是深度集成的参考模板**（基础路径路由 + 双层认证 + SSE/WS 透传），Dify、VS Code、JupyterLab 等任意 Docker 镜像均可接入。

| 工具 | 镜像 | 端口 | 启动命令 | 说明 |
|---|---|---|---|---|
| **opencode**（系统模板） | `ghcr.io/anomalyco/opencode:latest` | 4096 | `serve --mdns` | AI 辅助编程 IDE，Basic Auth 双层认证 |
| **VS Code**（系统模板） | `codercom/code-server:latest` | 8080 | `code-server --bind-addr 0.0.0.0:8080 --auth none` | 浏览器版 VS Code，经典编程教学环境 |
| **JupyterLab**（系统模板） | `jupyter/base-notebook:latest` | 8888 | —（`NOTEBOOK_ARGS` 关闭 token/password） | 数据科学 / Python 教学笔记本 |
| **Dify** | `langgenius/dify` | 3000 | — | LLM 应用搭建平台（LLMOps），用户可视化编排 AI 应用 |

> 上表仅为示例，管理员可填入任意镜像，不限于这些。模板按课程/分组选用：同一课程/分组的用户使用同一模板；不同课程可用不同模板，互不影响。

> **多端口场景**：一个模板可配置**主端口 + 任意数量的附加端口**。典型用法是「开发工具一个端口、运行/调试中的应用另占几个端口」——例如模板主端口 4096（opencode 开发工具），附加端口 3000/5173（用户前端 dev server）、8000（后端 API）。用户可同时访问多个服务，全部端口经根路径与 `/port/{port}/` 进入，共享同一容器、同一 volume，无需额外申请。

> **直接使用现成镜像，无需自建定制镜像**：平台只要求镜像暴露 HTTP 端口即可接入。管理员直接填写现有的 Docker 镜像（opencode 官方镜像、code-server、jupyter 等）即可，需要额外配置时通过模板的**启动命令 / 环境变量**完成，不需要自己写 Dockerfile。仓库 `images/student/` 仅为可选示例（在官方镜像上预装 python/node/git，内置 opencode 配置 `autoupdate: false`；其基础镜像仍为 `:latest`，生产应改为固定 tag）。

> **其它工具接入说明**：opencode 依赖容器内 `OPENCODE_SERVER_BASE_PATH` 环境变量自行处理 URL 前缀，代理无需改写。Dify、code-server 等若支持 URL 前缀 / 代理友好可直接套用；不支持前缀路由的工具需要在代理层做路径改写或独立反代配置（见「已知风险与 FAQ」）。

## 代理层要求

opencode web 大量使用 SSE（`/global/event`）与 WebSocket：

- Go `httputil.ReverseProxy` 设 `FlushInterval: -1`
- 长连接超时（SSE 可挂数小时），逐请求关闭缓冲
- 代理对容器做**健康检查后转发热启动**：`SyncStatus → EnsureRunning → WaitHealthy（15s）`，容器停止时秒级唤醒
- 上游统一带 Basic Auth（`opencode:{容器随机密码}`），WebSocket 经 `Hijack` 透传

> Dify、code-server 等同样重度依赖 SSE / WebSocket（Dify 实时消息、code-server 内置终端），上述 `FlushInterval: -1`、长连接超时与 nginx `proxy_buffering off` 的配置对它们同样适用。

## 安全设计

- 密码 argon2 哈希（明文存于 `password_plain` 供管理员查看；批量建号响应直接返回明文账号，由前端生成 `accounts.csv` 分发）；登录接口限流（每 IP 每分钟 10 次）
- JWT 短生命周期（30 分钟 access + 24h refresh，HttpOnly cookie）；代理只允许用户访问自己的容器，杜绝越权/SSRF
- 双层认证：建容器时生成随机密码注入 `OPENCODE_SERVER_USERNAME/PASSWORD`，代理自动带 Basic Auth 上游转发，浏览器无感；主端口与附加端口全部走同一代理，即使绕过代理直连容器端口也打不开（vscode 系统模板 `--auth none` 依赖该层兜底）
- user-net 为自定义 bridge；容器间隔离依赖**每容器独立随机密码**（代理注入），用户容器不挂 docker.sock，api 独享 socket
- 容器安全加固：CPU/内存/pids=128 限额、`no-new-privileges`、`CapDrop: ALL`
- `access_logs` 全量审计用户请求；生产环境 nginx 做 TLS 终止

> 注：`user-net` 未启用 ICC 隔离（`enable_icc` 保持默认），因为代理容器必须按容器名可达学生容器；跨容器横向访问被工具侧 Basic Auth 与随机密码阻断。如需更强的网络级隔离，需在宿主机网络层做 egress/ingress 控制。

## 资源估算（50 用户）

| 项 | 值 |
|---|---|
| 每容器限额 | 0.5 CPU / 1 GB 内存 / pids=128（CPU/内存限额模板与用户均可调，pids 固定 128） |
| 单机规格 | 16 vCPU / 64 GB → 可 50 个全量常驻；32 GB 机器必须开空闲停用 |
| 空闲回收 | 30 分钟无访问自动 `stop`（可配），访问时自动 `start`（秒级） |
| 批量建容器 | 一次性预拉镜像 + 并发 create（`BATCH_CONCURRENCY=5`，1~2s/个），50 个约 1 分钟内完成 |
| 磁盘 | 每用户 volume `code-{username}`(工作区) + `ocdata-{username}`(会话)，约 2 GB/人，50 人 ≈ 100 GB |

## 数据模型（PostgreSQL）

```
users            id, username(unique), password_hash, password_plain, role(admin/user),
                 status(active/disabled/expired), course(课程), expires_at,
                 cpu_limit, mem_limit, container_id, created_at, updated_at
user_containers  id, user_id(FK, unique), template_id, container_id, container_name,
                 status(pending/creating/running/stopped/error/removed),
                 internal_port(主端口), secret(容器随机密码), created_at, updated_at
image_templates  id, name(unique), image, internal_port(主端口), extra_ports(jsonb 附加端口),
                 envs(jsonb), cpu_limit, mem_limit, healthcheck_cmd,
                 workspace_dir, command(jsonb 启动命令), is_system, created_at
access_logs      id(bigserial), user_id, path, status, bytes, latency_ms, ts
```

## REST API 设计

所有平台接口位于 `/platform/` 前缀下，响应统一为 `{"data": ...}` / `{"error": "..."}`。

| 模块 | 接口 |
|---|---|
| 认证 | `POST /platform/auth/login`（限流）、`POST /platform/auth/refresh`、`GET /platform/auth/logout`、`GET /platform/auth/me`（返回 `user` + `container`（含主/附加端口））、`POST /platform/auth/change-password`（用户自助改密码，新密码 ≥ 8 位） |
| 用户 | `POST /platform/admin/users/batch`（生成 N 个/显式用户名列表，可带 `course/expires_in_days/限额`，响应含明文账号）、`GET /platform/admin/users`（含当前密码）、`PATCH /platform/admin/users/{id}`（改密码/禁用/延长到期/改课程/限额）、`DELETE /platform/admin/users/{id}`（级联移除容器）、`GET /platform/admin/users/export`（JSON 账号清单）、`POST /platform/admin/users/batch/action`（按 `user_ids` 批量 `delete/restart/stop`） |
| 容器 | `POST /platform/admin/containers/batch`（按 `template_id` + `user_ids`，`force` 可重建）、`GET /platform/admin/containers`（实时状态调和）、`POST /platform/admin/containers/{id}/start\|stop\|restart\|remove`、`GET /platform/admin/containers/{id}/stats`、`GET /platform/admin/containers/stats/all`（各容器实时 Docker stats） |
| 模板 | `GET/POST /platform/admin/templates`、`GET/PUT/DELETE /platform/admin/templates/{id}`（`internal_port` 主端口 + `extra_ports[]` 附加端口；系统模板不可删） |
| 统计 | `GET /platform/admin/stats/dashboard`（用户/容器状态分布、在线用户（5 分钟内）、24h 请求趋势、CPU/内存实时聚合、课程分布、模板数、空闲超时配置） |
| 代理 | `/*` 根路径（JWT 身份识别 + ReverseProxy，学生请求转发到其容器，支持 `/port/{port}/` 附加端口）；`/portal`、`/admin`、`/static/*` 恒为平台 SPA |
| 健康检查 | `GET /api/health` |

## 项目结构

```
DevCapsule/
├── docker-compose.yml        # 生产部署：api + nginx + postgres
├── .env.example              # compose 部署配置模板（JWT_SECRET / ADMIN_PASSWORD 必填）
├── backend/
│   ├── Dockerfile            # 单二进制镜像（前端 dist 在构建时 embed）
│   ├── cmd/server/main.go    # 入口：配置 → 数据库迁移 → Docker → 启动
│   └── internal/
│       ├── api/              # 路由、中间件、handler（含 SPA 静态托管；web/dist 为 embed 前端产物）
│       ├── auth/             # JWT（30min access + 24h refresh）、argon2 密码哈希
│       ├── batch/            # 批量账号生成（课程名推导前缀、随机密码、CSV 导出）
│       ├── config/           # 环境变量配置
│       ├── docker/           # Docker SDK 封装、编排（Provision/IdleStop/Expire/Reconcile/Stats/健康检查）
│       ├── model/            # 数据模型
│       ├── proxy/            # 反向代理（SSE/WS 透传、Basic Auth 上游、/port/ 路由、访问日志）
│       └── store/            # 存储接口 + PostgreSQL / 内存实现
├── frontend/                 # Vue3 管理台 + 用户门户（vite 构建到 backend/internal/api/web/dist）
├── images/student/           # （可选）自定义工具镜像示例——平台直接用现成镜像，无需自建
├── deploy/nginx.conf         # 生产 nginx 反代（SSE/WS 透传）
└── e2e/run.sh                # 端到端测试脚本（登录→建号→建容器→代理→改密→统计）
```

## 技术选型

- **后端**：Go（标准库 net/http + Docker SDK + pgx + golang-jwt + argon2）
- **前端**：Vue3 + Vite + vue-router（管理台 + 用户门户两个路由区），构建产物 `embed` 进 Go 二进制单文件分发
- **数据库**：PostgreSQL 17（存储层为接口，本地开发可切内存实现）
- **部署**：docker-compose（`api`、`nginx`、`postgres`）+ `.env` 管理密钥；api 单二进制含前端静态资源

## 开发与测试

```bash
cd backend
go test ./...        # 单元 + 集成测试（代理 SSE/WS、鉴权、批量流程；Docker 集成测试可用 TEST_DOCKER=0 跳过）
go vet ./...         # 静态检查
go build ./cmd/server

# 端到端测试（需 docker compose up -d 且已建 user-net；先构建 e2e 用镜像）
docker build -t devcapsule/student:1 images/student
bash e2e/run.sh      # 覆盖：登录 → 建模板 → 批量建号 → 批量建容器 → 学生访问代理 → 批量操作 → 改密 → Dashboard 统计
```

当前实现状态：
- ✅ Go 后端：认证（JWT + 限流）、批量建号、模板（多端口/环境变量/启动命令）、容器编排（幂等批量/空闲停/到期停/状态调和/实时 stats）、反向代理（SSE/WS、双层认证、多端口路由、访问日志）、后台周期任务
- ✅ 前端：Vue3 管理台（总览 / 用户与容器 / 镜像模板 / 使用帮助）+ 用户门户（查看状态、打开环境、自助改密），构建产物 embed 进 Go 二进制
- ✅ 部署：docker-compose（api/nginx/postgres）+ `.env`、`deploy/nginx.conf`、e2e 脚本
- 🚧 待完善：生产 TLS 全链路验证、通用工具路径改写（见里程碑）

## 已知风险与 FAQ

1. **opencode 版本**：系统模板默认 `:latest`，`--base-path`/`OPENCODE_SERVER_BASE_PATH` 较新，生产建议锁定固定版本并 `autoupdate: false`，升级前先在测试环境验证代理
2. **SSE 断流**：经 nginx + Go 反代后 stream 稳定性需重点测试（务必 `proxy_buffering off`）
3. **macOS 开发机**：Docker Desktop 需手动提高 VM 内存/CPU，一次跑 50 容器本地吃力 → 生产建议 Linux 服务器
4. **成本**：AI 用量统计不在平台范围，LLM Key 由各镜像/工具自行配置，平台不参与
5. **用户终端外网访问**：opencode 内置终端允许用户安装依赖/访问外网，属预期行为；如需收紧用网络层 egress 控制
6. **任意镜像兼容性**：平台本身只要求镜像暴露 HTTP 端口即可接入，且支持多端口。opencode 因支持基础路径路由、深度集成而开箱即用；code-server（系统模板通过 `--auth none` + 代理双层认证接入）、Dify 等不支持前缀的工具，需要代理层路径改写或独立反代配置（基础路径剥离、静态资源相对路径、Cookie 域等，见里程碑 M5）

## 里程碑

- **M1**（✅ 完成）：Go 后端——认证、批量建号、容器编排、模板、统计、后台周期任务
- **M2**（✅ 完成）：反向代理（SSE/WS 透传、双层认证、多端口路由）+ 用户门户
- **M3**（✅ 完成）：管理台（总览 / 用户与容器 / 模板 / 帮助）+ 空闲自动停 + 账号过期
- **M4**（🟡 部分完成）：登录限流、`access_logs` 审计已实现；TLS 由 nginx 承担，生产全链路验证待做
- **M5**（🚧 待做）：通用工具路径改写——Dify / JupyterLab 等不支持前缀路由的工具，代理层路径改写与独立鉴权适配
