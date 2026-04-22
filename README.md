# tmbark-notifier

为 Teslamate 自建用户打造的轻量通知工具。通过 MQTT 监听车辆状态，结合数据库查询，在行程结束时自动推送美观格式的通知到「特特管家」iOS App。持续优化中...

## 功能特性

- 行程、充电信息自动推送通知
- 包含行驶时间、距离、电量变化等详细信息
- Bark 行程通知支持展开查看真实轨迹地图图，点击进入自建详情页
- Bark 轨迹图默认使用 OSM，可通过配置切换到高德静态图
- 详情页支持 OSM / 高德地图顶部切换，默认 OSM
- 集成到现有的 Teslamate docker-compose 环境中
- 轻量级，资源占用低
- 支持多平台（amd64、arm64）

## Docker 部署

### 前提条件

- 已运行 Teslamate（包含 database 和 mosquitto 服务）
- 基于 Teslamate v3.0.0 开发，其他版本未经过验证（有使用v2.1的车友反馈可以正常推送）
- Docker 和 Docker Compose 已安装

### 安装步骤

1. **准备 Docker 专用环境变量**

   复制 `/.env.docker.example` 为 `/.env.docker`，填入你的 Bark、数据库、MQTT 等真实配置。
   `/.env` 建议只保留给本机 `go run ./cmd/app` 联调使用。

2. **编辑你的 `docker-compose.yml` 文件**，在 `services` 部分添加以下内容（参考 `docker-compose.example.yml`）：

   ```yaml
   services:
     # ... 现有的 Teslamate 服务 ...
     
     tmbark-notifier-car1:
       image: tmbark-notifier:local
       restart: always
       ports:
         - "8080:8080"
       env_file:
         - ./.env.docker
       environment:
         - CAR_ID=1 # 每个容器单独指定车辆
         - PUBLIC_BASE_URL=http://your-host-or-domain:8080 # Bark 所在手机可以访问到的地址
       depends_on:
         - database #依赖数据库服务，同上HOST
         - mosquitto #依赖MQTT服务，同上HOST

     # 如有多辆车，请再新增容器，修改CAR_ID（未经验证）
   ```

3. **配置说明**

   | 环境变量 | 说明 | 必填 | 默认值 |
   |---------|------|------|--------|
   | API_TOKEN | 特特管家的完整推送 API 地址 | 是 | - |
   | DATABASE_HOST | 数据库主机名 | 否 | database |
   | DATABASE_USER | 数据库用户名 | 否 | teslamate |
   | DATABASE_PASS | 数据库密码 | 是 | - |
   | DATABASE_NAME | 数据库名称 | 否 | teslamate |
   | MQTT_HOST | MQTT 主机名 | 否 | mosquitto |
   | CAR_ID | 车辆 ID，建议在 `docker-compose.yml` 中按服务覆盖 | 否 | 1 |
   | PUSH_DEBOUNCE_SECONDS | 推送防抖动初始时间（秒） | 否 | 5 |
   | WEB_PORT | 内置详情页服务端口，Docker 通常固定为 8080 | 否 | 8080 |
   | PUBLIC_BASE_URL | 手机可访问的详情页基础地址，用于 Bark 图和点击详情页，建议在 `docker-compose.yml` 中按服务覆盖 | Bark 轨迹图必填 | - |
   | WEB_URL_SECRET | 详情页签名密钥，建议设置 | 否 | - |
   | TRACK_MAX_POINTS | 轨迹图采样点数上限 | 否 | 180 |
   | BARK_MAP_PROVIDER | Bark 轨迹图 provider，可选 `osm` / `amap` | 否 | `osm` |
   | DETAIL_MAP_DEFAULT_PROVIDER | 详情页默认 provider，可选 `osm` / `amap` | 否 | `osm` |
   | DETAIL_MAP_ENABLED_PROVIDERS | 详情页顶部允许切换的 provider 列表 | 否 | `osm,amap` |
   | OSM_TILE_URL | OSM 底图模板地址 | 否 | `https://tile.openstreetmap.org/{z}/{x}/{y}.png` |
| OSM_TILE_USER_AGENT | 服务端请求 OSM 底图时使用的 User-Agent | 否 | `tmbark-notifier/1.0 (+https://github.com/CandyLion/tmbark-notifier)` |
   | OSM_TILE_CACHE_HOURS | OSM 瓦片本地缓存小时数 | 否 | 168 |
   | AMAP_STATIC_MAP_URL | 高德静态地图接口地址 | 否 | `https://restapi.amap.com/v3/staticmap` |
   | AMAP_WEB_SERVICE_KEY | 高德 Web服务 API Key，用于 Bark 高德静态图 | Bark 选 `amap` 时必填 | - |
   | AMAP_JS_KEY | 高德 JS API Key，用于详情页高德地图 | 详情页启用高德时必填 | - |
   | AMAP_JS_SECURITY_CODE | 高德 JS 安全密钥，新申请的 JS Key 常需要 | 否 | - |
   | AMAP_JS_VERSION | 高德 JS API 版本 | 否 | `2.0` |

   说明：
   `MAP_TILE_URL` / `MAP_TILE_USER_AGENT` / `MAP_TILE_CACHE_HOURS` 旧变量仍兼容，但推荐改用 `OSM_TILE_*` 新名字。

4. **启动服务**

   如果你已经有正在运行的容器，使用以下命令（只会更新有变化的容器）：

   ```bash
   docker compose up -d
   ```

   Docker Compose 会自动检测变化，只重新创建新增的 `tmbark-notifier-car1` 容器，不会影响其他正在运行的容器。

   如果需要更新镜像版本，请执行以下命令：

   ```bash
   docker compose build tmbark-notifier-car1
   docker compose up -d tmbark-notifier-car1
   ```

5. **查看日志**

   ```bash
   docker compose logs -f tmbark-notifier-car1
   ```

### 发布多架构镜像

- 推荐使用主 `Dockerfile` 从 GitHub tag 远程构建并直接推送到 Docker Hub，这样不会把你本地的 `.env`、`.env.docker`、`dist/` 等文件带入发布上下文
- `Dockerfile.local` 只适合本机快速打包测试，不建议用于公开发布

```bash
# 登录 Docker Hub
docker login

# 确保 buildx builder 已就绪
docker buildx create --name tmbark-builder --driver docker-container --use
docker buildx inspect --bootstrap

# 从 GitHub 的 v1.1.0 tag 远程构建并推送 amd64 + arm64
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t <dockerhub-user>/tmbark-notifier:v1.1.0 \
  -t <dockerhub-user>/tmbark-notifier:latest \
  --push \
  https://github.com/CandyLion/tmbark-notifier.git#v1.1.0
```

说明：
- 如果你只是本机测试，可继续使用 `tmbark-notifier:local`
- 如果你需要发布到 Docker Hub，建议优先使用上面的远程 GitHub tag 构建方式
- 若未来发新版本，把 `v1.1.0` 替换成新的 tag 即可

### Bark 轨迹图说明

- 只有 Bark 通知会附带真实轨迹地图图和详情页点击链接
- 需要正确配置 `PUBLIC_BASE_URL`，并确保手机能访问该地址
- 建议同时设置 `WEB_URL_SECRET`，这样通知里的详情页和轨迹图地址会自动带签名
- Bark 轨迹图默认使用 `OSM`，可通过 `BARK_MAP_PROVIDER=amap` 切换到高德静态图
- 详情页默认使用 `OSM`，如果同时配置了高德 Key，并在 `DETAIL_MAP_ENABLED_PROVIDERS` 中包含 `amap`，页面顶部会出现 `OSM / 高德` 切换
- OSM 官方瓦片不需要 API Key，但不适合无限量高频调用；当前实现会在本地缓存瓦片 7 天，并在拉取失败时回退成纯轨迹线条图
- 高德静态图和详情页高德地图分别使用 `AMAP_WEB_SERVICE_KEY` 和 `AMAP_JS_KEY`

## 获取 API_TOKEN

1. 在「特特管家」iOS App 中获取完整的推送 API 地址
2. 将 API 填入 `API_TOKEN` 环境变量

## 版本说明

- `v1.0.0` - 初始版本，支持行程结束、充电结束、哨兵模式状态变更自动通知
- `v1.0.1` - 移除哨兵通知，新增充电开始通知，调整通知内容样式，优化推送逻辑
- `v1.1.0` - 新增 Bark 轨迹图与自建详情页，支持 OSM / 高德 provider 切换，补充 Docker / 本机双环境配置与多架构镜像构建说明

## 技术栈

- Go 语言
- GORM（数据库 ORM）
- paho.mqtt.golang（MQTT 客户端）

## 本地开发

```bash
# 克隆项目
git clone https://github.com/CandyLion/tmbark-notifier.git

# 配置本机联调环境变量
cp .env.example .env
# 编辑 .env 文件

# 运行
go run ./cmd/app
```

## License

MIT License
