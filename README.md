# Paynet · TLS + TCP + Protobuf 通讯库

该仓库实现了一套可复用的通讯栈，结合 TLS1.3、长度前缀帧和 Protobuf，帮助支付设备/盒子与云端服务安全、低成本地保持长连接。库同时提供 Go 服务端与客户端 SDK、证书脚本以及接入文档，便于项目快速落地。

---

## 功能亮点

- 🔒 **安全链路**：默认 TLS1.3 + 可选 mTLS，支持 Session Resumption、可插拔 CA。
- 🔁 **长连接复用**：单 TCP 连接承载多种业务命令、心跳、查询等消息。
- 📨 **统一帧格式**：`Len(4B) + Type(1B) + Protobuf Payload`，简化多语言接入。
- ⚙️ **可扩展注册表**：`paynet.Registry` 绑定 Type ↔ Handler，业务随时扩展 proto。
- ♻️ **自动重连/心跳**：客户端内置指数退避、定时心跳、上下线回调。
- 🛠️ **工具配套**：证书脚本、示例 server/client、接入指引文档齐备。

---

## 仓库结构

| 路径 | 说明 |
| --- | --- |
| `pkg/paynet/` | 核心库（frame、session、client/server、Registry 等） |
| `internal/pb/` | 由 `paybox.proto` 生成的 Go 代码 |
| `cmd/paynetd/` | 示例服务端：注册、登录、心跳、命令处理 |
| `cmd/paynetcli/` | 示例客户端：自动心跳、命令执行、回执 |
| `scripts/` | `gen_certs.sh`、`gen_device_cert.sh` 等辅助脚本 |
| `CLIENT_INTEGRATION.md` | 面向合作方的接入/部署手册 |
| `EXAMPLE.md` | 最小可运行示例与代码片段 |

---

## 快速开始

1. **安装依赖**
   ```bash
   go mod download
   ```
2. **生成 TLS 证书**
   ```bash
   scripts/gen_certs.sh paynet.local device-demo
   # 如需更多设备证书
   scripts/gen_device_cert.sh device-002
   ```
3. **生成 Protobuf 代码**
   ```bash
   protoc --go_out=internal/pb --go_opt=paths=source_relative paybox.proto
   ```
4. **运行示例服务端**
   ```bash
   go run ./cmd/paynetd
   ```
   或在代码中手动控制生命周期：
   ```go
   srv, _ := paynet.NewServer(cfg)
   if err := srv.Start(); err != nil {
       log.Fatal(err)
   }
   defer srv.Stop()
   // ... 可在其他 goroutine 中调用 srv.Wait()
   ```
5. **运行示例客户端**
   ```bash
   go run ./cmd/paynetcli
   ```
   同样支持可控生命周期：
   ```go
   cli, _ := paynet.NewClient(cfg)
   if err := cli.StartContext(ctx, flow); err != nil { log.Fatal(err) }
   defer cli.Stop()
   if err := cli.Wait(); err != nil { log.Fatal(err) }
   ```
   查看终端日志即可观察 Register → Login → Heartbeat → CoinCommand 等交互。

> 详细代码讲解见 `EXAMPLE.md`，业务对接流程/部署注意事项参考 `CLIENT_INTEGRATION.md`。

---

## 在项目中使用

1. 将 `pkg/paynet` 引入业务模块，复用公共 Type 定义（`pkg/paynet/message_ids.go`）或自定义映射。
2. 业务侧根据各自的 `.proto` 生成 Go/C 等语言的代码，并通过 `paynet.Registry` 注册 Handler。
3. 服务端使用 `paynet.NewServer`，配置 TLS、最大帧长、ReadTimeout 等即可监听；客户端通过 `paynet.NewClient` + `LoginFlow` 完成注册/登录。
4. 可选用 `scripts/gen_device_cert.sh` 为每台设备签发证书，或接入已有 PKI 系统。
5. **业务鉴权/白名单仍需在 Handler 中实现**：本库负责链路加密与身份凭证校验，不会替业务层做注册、鉴权、幂等等判断；请在具体业务逻辑里决定哪些设备可执行命令。

---

## 参考与支持

- `CLIENT_INTEGRATION.md`：部署、配置、验收 checklist。
- `EXAMPLE.md`：最小 demo 代码。
- `paybox.proto`：协议定义，需保持服务端与客户端一致。
- 需要更多语言 SDK、监控指标、或自动化工具，可在 Issues 中提出。

欢迎 Fork / PR / Issue，一起完善这一套安全通讯方案。***
