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
- `GET /api/v1/capabilities` — 能力契约（JSON Schema 2020-12）

## 能力契约

`pkg/capability` 定义词汇表，`pkg/ffmpeg/capability.go` 就地声明各函数对产物的
承诺，`internal/api/capability.go` 把它们组合成本服务的契约。

改 `BuildTranscodeArgs` / `CalculateTargetDimensions` / `CalculateTargetBitrate` /
`BuildExtractFrameArgs` 时**必须同步复核** `pkg/ffmpeg/capability.go` —— 声明与
实现放在同一个包里就是为了让两者出现在同一个 diff 里。

三条硬规则：

1. **表达不了就标注，不要伪装成校验。** JSON Schema 关键字能表达的用关键字，
   跨字段关系、哪个属性才是权威闸门、某字段为何被接受又被忽略，一律走 `x-` 注解
2. **`Narrow` 永远不改 guarantee。** 三档按可接受产物集合嵌套（exact ⊂ best_effort
   ⊂ none），弱化是在扩大集合、会击穿依赖强承诺的消费方；强化则是策略层替实现层
   许一个兑现不了的承诺。字段未导出，由编译器守这条规则
3. **渲染必须确定性。** 用 `encoding/json`（它排序 map 键），有序位置一律用 slice。
   消费方会缓存并对 ETag，非确定性渲染会让缓存静默失效

## 远端仓库

- **github**: `git@github.com:yuerchu/media-gateway.git`
- **origin** (Gitea): `git@git.yxqi.cn:disknext/media-gateway.git`

推送策略：**只推 github**。自建 Gitea 正在迁往 GitLab，数据尚未迁移，`origin`
暂不可依赖。消费方（foxline-pro-media-middleware）也是从 github 路径拉取本模块。
