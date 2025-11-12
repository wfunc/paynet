# 客户端接入与部署指引

面向需要对接支付盒子安全通信链路（TLS/SSL + TCP + Protobuf）的合作方，本文提供服务端部署、客户端集成、协议细节及验收流程，确保上线后可稳定收发 Register/Login/Heartbeat/Coin 等业务消息。

---

## 1. 总体架构与角色

| 角色 | 说明 | 核心职责 |
| --- | --- | --- |
| `paynet-server` | Go 实现的 TLS 服务端 | 监听 443/自定义端口、校验证书、调度Protobuf消息到业务 Handler |
| `paynet-client-sdk` | Go 客户端库（可由其他语言移植） | 建立安全长连接、自动重连/心跳、暴露业务收发接口 |
| 业务服务 (Ops) | 业务后端 | 通过 SDK 向设备下发命令、订阅执行结果 |
| 设备/客户端 | 4G/宽带终端 | 运行 paynet 客户端，完成注册、登录、心跳、执行命令 |

链路特点：单条 TLS1.3 长连接、多条业务命令复用、帧格式为 `Len(4B)+Type(1B)+Payload`，payload 为对应 proto 消息序列化结果。

---

## 2. 部署前置要求

1. **网络**：固定公网 IP 或接入负载均衡；允许入站 TCP 443（或配置端口），设备外网需放行同端口出站。
2. **证书**：服务端使用正式证书；推荐开启双向认证（mTLS），设备出厂预置 CA 与客户端证书。
3. **系统环境**：Linux x86_64，Go 1.21+，可访问 Redis（如需幂等存储）。
4. **依赖文件**：
   - `paybox.proto` 及生成的 `payboxpb` Go 包
   - TLS 证书：`server.crt`, `server.key`, （可选）`ca.crt`, `device.crt`, `device.key`
   - `config.yaml`：服务端/客户端配置

---

## 3. 服务端部署步骤

1. **拉取代码与依赖**
   ```bash
   git clone <repo> paynet && cd paynet
   go mod download
   ```
2. **生成 Protobuf**
   ```bash
   make proto  # 或 paynetctl proto gen
   ```
3. **配置文件模板**
   ```yaml
   server:
     addr: ":443"
     max_frame: 65535
     heartbeat_expect: 70s
     tls:
       cert: /etc/paynet/server.crt
       key: /etc/paynet/server.key
       client_ca: /etc/paynet/ca.crt
       mutual_auth: true
   dispatcher:
     workers: 32
     queue: 2048
   idempotency:
     backend: redis
     dsn: redis://user:pass@host:6379/0
   ```
4. **注册业务 Handler**（示例 `cmd/paynetd/main.go`）
   ```go
   func init() {
       paynet.MustRegister(paynet.Meta{
           Type: 0x01,
           Name: "RegisterReq",
           Factory: func() proto.Message { return &payboxpb.RegisterReq{} },
           Handler: paynet.HandlerFunc(handleRegister),
       })
       // 其他类型同理
   }
   ```
5. **编译并运行**
   ```bash
   go build ./cmd/paynetd
   ./paynetd -config /etc/paynet/config.yaml
   ```
6. **监控与日志**
   - 暴露 `:9090/metrics` (Prometheus) 监控连接数、心跳延迟、帧吞吐
   - `journalctl -u paynetd` 或配置到 ELK

---

## 4. 客户端集成流程

### 4.1 准备

1. 引入 `paynet-client-sdk`（Go 模块示例）
   ```bash
   go get github.com/yourorg/paynet
   ```
2. 使用 `paynetctl proto gen` 生成各自业务 proto 代码并放入 `bizpb` 包。
3. 配置 TLS：
   ```yaml
   client:
     addr: "paynet.example.com:443"
     tls:
       ca: /opt/paynet/ca.crt
       cert: /opt/paynet/device.crt
       key: /opt/paynet/device.key
     reconnect:
       base: 1s
       max: 64s
     heartbeat_send: 60s
   ```

### 4.2 初始化与连接

```go
cli := paynet.NewClient(paynet.ClientConfig{...})

cli.Subscribe(0x04, paynet.HandlerFunc(handleCoinCommand))
cli.Subscribe(0x05, paynet.HandlerFunc(handleCoinAck))

flow := paynet.LoginFlow{
    Register: &payboxpb.RegisterReq{DeviceSn: "...", Model: "..."},
    Login:    &payboxpb.LoginReq{DeviceId: "...", Token: "...", Version: "1.0.0"},
}

if err := cli.Run(ctx, flow); err != nil {
    log.Fatalf("connect failed: %v", err)
}
// 若需后台运行：
// if err := cli.StartContext(ctx, flow); err != nil { ... }
// defer cli.Stop()
// go func() { _ = cli.Wait() }()
```

### 4.3 业务调用

- **发送心跳**（自动）：SDK 每 60 秒发送 `Ping`，可通过 `paynet.WithHeartbeatInterval` 调整。
- **下发命令**：
  ```go
  err := cli.Send(ctx, &payboxpb.CoinCommand{
      OrderId: "202402010001",
      Coins:   10,
      Timeout: 90,
  })
  ```
- **处理回执**：在 `handleCoinAck` 中根据 `state` 更新业务系统，并可触发 `QueryReq`。

---

## 5. 协议速查

| Type (十六进制) | 方向 | Protobuf 消息 | 说明 |
| --- | --- | --- | --- |
| `0x01` | Client → Server | `RegisterReq` | 设备注册 |
| `0x81` | Server → Client | `RegisterRsp` | 注册响应 |
| `0x02` | Client → Server | `LoginReq` | 登录 |
| `0x82` | Server → Client | `LoginRsp` | 登录响应（含 resume_token） |
| `0x03` | 双向 | `Ping`/`Pong` | 心跳 |
| `0x04` | Server → Client | `CoinCommand` | 上币命令 |
| `0x84` | Client → Server | `CoinAck` | 命令回执/进度/结果 |
| `0x05` | 双向 | `QueryReq`/`QueryRsp` | 查询订单状态 |
| `0xFF` | 双向 | `ErrorMsg` | 错误通知 |

> Type 对应关系可按模块扩展；建议保留 `0xF0~0xFF` 作为诊断/保留字段。

---

## 6. 错误码与重试

- `ResultCode` 详见 `paybox.proto`，客户端应对 `UNAUTHORIZED` 触发重新登录，对 `ORDER_DUPLICATED` 忽略重复命令。
- 网络错误/握手失败：客户端会指数退避重连（1s → 64s），可通过配置项限制最大退避时间。
- 消息级错误：服务端返回 `ErrorMsg`（code、msg），客户端可根据 `code` 决定是否重试或人工介入。

> **重要说明**：通讯层仅负责 TLS/mTLS、帧封装与消息分发；是否允许设备注册、验证 Token/白名单、限制命令下发等业务规则，需要在各自的 Handler/业务服务中实现。本库不会替代业务鉴权，请务必结合实际场景补齐安全策略。

---

## 7. 验收测试清单

1. **TLS 验证**：无证书或错误证书连接应被拒绝；证书到期提醒。
2. **注册/登录**：首次注册成功、重复注册返回已存在；登录状态可恢复。
3. **心跳**：60s 内不发 Ping 服务端断开；断线后客户端可自动重连。
4. **业务命令**：下发 `CoinCommand`，设备执行并多次反馈 `CoinAck`，最终状态一致。
5. **查询**：断线后恢复连接，可继续查询历史订单。
6. **幂等**：重复订单号只执行一次，`CoinAck` 状态可重复上报。

---

## 8. 支持与升级

- 问题反馈：support@paynet.example.com，提供 `ConnID`、时间、Type、日志片段。
- 版本管理：服务端与客户端紧跟语义化版本（例如 v1.2.0），重大变更提前 2 周公告。
- Proto 变更：新增字段使用 proto3 optional，保持向后兼容；若需新增消息/Type，统一在注册表登记。

---

## 9. 附录

### 9.1 paynetctl 常用命令

```bash
paynetctl proto init            # 初始化 proto 工程
paynetctl proto gen             # 生成 Go 代码
paynetctl registry scaffold \
  --type 0x40 --message CoinCommand --handler handleCoinCommand
paynetctl tls gencert --cn device001 --out ./certs
```

### 9.2 典型时序

```
Device SDK -> paynet-server: TLS1.3 handshake (mTLS)
Device SDK -> paynet-server: RegisterReq
paynet-server -> Device SDK: RegisterRsp
Device SDK -> paynet-server: LoginReq (+resume_token)
paynet-server -> Device SDK: LoginRsp
Device SDK <-> paynet-server: Ping/Pong (60s)
paynet-server -> Device SDK: CoinCommand
Device SDK -> paynet-server: CoinAck (ACCEPTED → RUNNING → DONE)
```

---

如需定制更多业务流程、扩展 Type 或跨语言 SDK，请联系技术支持团队。我们也可提供现场联调脚本与抓包模板，确保上线顺利。
