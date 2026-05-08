# outlook-imap-service

## 职责

`outlook-imap-service` 负责 Outlook 邮箱 OTP。当前实现通过 Microsoft Graph 持续轮询收件箱，并向调用方返回匹配邮箱地址和主题关键词的 6 位验证码。

服务会按邮箱地址缓存最近一条 OTP，`WaitForEmail` 取走后立即清空该邮箱的缓存；如果请求先到，则等待后续邮件并在命中后返回。

邮箱地址由 `account-db` 生成，编排服务调用 `WaitForEmail` 等待注册 OTP。

## 容器参数

| 参数 | 默认值 | 说明 |
| --- | --- | --- |
| `OUTLOOK_REFRESH_TOKEN_FILE` | `tokens/outlook_refresh_token` | refresh token 持久化文件 |
| `LISTEN_ADDR` | `:50053` | gRPC 监听地址 |

## OAuth 初始化

首次运行前执行 device flow，生成并持久化 refresh token：

```bash
go run ./tools/auth.go
```

默认写入 `tokens/outlook_refresh_token`，该目录已被 `.gitignore` 排除。需要自定义路径时：

```bash
go run ./tools/auth.go --token-file /path/to/outlook_refresh_token
```

服务启动后会从 token 文件读取 refresh token，并在后台定时刷新 Microsoft Graph access token，避免运行过程中 access token 过期。

## gRPC 接口

Proto: `proto/email.proto`

```proto
service EmailService {
  rpc WaitForEmail(WaitForEmailRequest) returns (WaitForEmailResponse);
}
```

等待 ChatGPT 邮件 OTP：

```bash
grpcurl -plaintext \
  -import-path proto -proto email.proto \
  -d '{"email_address":"user+abc123@example.com","subject_keyword":"ChatGPT","timeout_seconds":60}' \
  127.0.0.1:50053 email.EmailService/WaitForEmail
```

## 运行

```bash
docker compose up -d --build outlook-imap-service
```
