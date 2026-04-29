# media-gateway

通用媒体处理服务，提供视频转码、缩略图生成、元数据提取、MIME 检测等能力。

## 项目结构

- `pkg/` — 公开 Go 包，可被其他 Go 项目直接 `go get` 导入
- `internal/` — HTTP 服务层，不可外部导入
- `cmd/media-gateway/` — HTTP 服务入口

## 开发环境

- Go 1.25+
- 外部依赖：ffmpeg、ffprobe（用于视频处理）

## 运行

```bash
# 环境变量配置
export MG_LISTEN=":8190"
export MG_AUTH_SECRET="your-hmac-secret"
export MG_FFMPEG_FFMPEG_PATH="ffmpeg"
export MG_FFMPEG_FFPROBE_PATH="ffprobe"

go run ./cmd/media-gateway
```

## 消费方式

### 作为 Go module（零延迟，foxline 等 Go 项目用）

```go
import (
    "github.com/yuerchu/media-gateway/pkg/mime"
    "github.com/yuerchu/media-gateway/pkg/imagemeta"
    "github.com/yuerchu/media-gateway/pkg/ffmpeg"
)
```

### 作为 HTTP 服务（DiskNext 等非 Go 项目用）

- `POST /api/v1/detect-mime` — MIME 魔数检测
- `POST /api/v1/image-meta` — 图片尺寸提取
- `POST /api/v1/tasks` — 异步任务（thumbnail/metadata/transcode）
- `GET /api/v1/tasks/:id` — 查询任务状态
- `DELETE /api/v1/tasks/:id` — 取消任务

## 远端仓库

- **origin** (Gitea): `git@git.yxqi.cn:disknext/media-gateway.git`
- **github**: `git@github.com:yuerchu/media-gateway.git`

推送策略：默认推 Gitea。
