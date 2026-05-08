# outlook-imap-service

## 职责

`outlook-imap-service` 负责 Outlook 邮箱 OTP。当前实现通过 Microsoft Graph 轮询收件箱，并向调用方返回匹配邮箱地址和主题关键词的 6 位验证码。

`browser-reg` 会在注册请求内调用 `WaitForEmail`，默认最多等待 2 次，每次 60 秒。

## 容器参数

| 参数 | 默认值 | 说明 |
| --- | --- | --- |
| `OUTLOOK_EMAIL` | `test@outlook.com` | Outlook 主邮箱 |
| `OUTLOOK_REFRESH_TOKEN` | 空 | Graph OAuth refresh token |
| `OUTLOOK_AUTH_SCOPE` | `https://graph.microsoft.com/Mail.Read` | OAuth scope |
| `LISTEN_ADDR` | `:50053` | gRPC 监听地址 |

## gRPC 接口

Proto: `proto/email.proto`

```proto
service EmailService {
  rpc GetEmail(GetEmailRequest) returns (GetEmailResponse);
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
