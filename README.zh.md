# webchat

[English](README.md) | **中文**

一个基于 Go 的轻量实时聊天室示例：Gin 提供 HTTP 服务，Melody 提供 WebSocket 管理，前端为单文件 `index.html`（通过 `embed` 内嵌到二进制）。

## 功能

- 实时群聊（WebSocket）
- 发送文本消息
- 发送文件/图片（HTTP 上传，聊天室内广播并提供下载链接）
- 粘贴上传（截图/剪贴板文件）
- 本地聊天记录（localStorage）、搜索、清空
- 未读计数 + 浏览器通知（可选）
- 用户区分：使用浏览器“指纹”生成 `userId`（本地持久化），昵称仅用于展示

## 快速开始

### 环境要求

- Go（建议与 `go.mod` 中版本兼容）

### 运行

```bash
go run .
```

然后访问：

- `http://localhost:5000`

## 路由与协议

### HTTP

- `GET /`：返回前端页面（内嵌 `index.html`）
- `POST /upload`：上传文件（`multipart/form-data`）
  - 字段：`file`（文件）、`userId`、`userName`
- `GET /files/:filename`：下载文件

### WebSocket

- `GET /ws?userId=...&userName=...`
- 客户端发送：
  - 文本：`{"type":"text","userId":"...","userName":"...","content":"..."}`
  - 心跳：`{"type":"ping"}`
- 服务端广播：
  - 用户消息：`{"type":"user","userId":"...","userName":"...","content":"..."}`
  - 系统消息：`{"type":"system","content":"..."}`
  - 文件消息：`{"type":"file","filename":"...","filesize":123,"userId":"...","userName":"..."}`

## 文件存储

- 上传文件保存到：`./files/`
- 同名文件会自动追加数字后缀：`name(1).ext`、`name(2).ext` ...
- 下载链接为：`/files/{filename}`（前端会对 `filename` 做 URL 编码）

## 安全与限制（重要）

该项目偏演示用途，未包含登录鉴权/权限控制，请勿直接暴露到公网生产环境。

- 文件名校验：禁止 `..` 及路径分隔符，防止目录穿越
- 上传大小限制：默认 `20MiB`
- WebSocket Origin 校验：默认仅允许同源（同 Host 的 `http/https` Origin）
- 下载响应头：设置 `X-Content-Type-Options: nosniff`；非图片使用附件下载方式返回

如需公网部署，建议至少补齐：鉴权、限流、审计日志、文件类型白名单/病毒扫描、存储改为对象存储等。

## 开发说明

- 后端入口：`main.go`
- 前端页面：`index.html`（由后端 `embed` 内嵌提供）

