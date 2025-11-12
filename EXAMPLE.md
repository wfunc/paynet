# Paynet 通讯库示例

本示例演示如何在自己的项目中使用 `pkg/paynet` 完成 TLS + TCP + Protobuf 的双向通讯。所有代码均可直接复制到业务仓库。完整可运行的 Demo 分别位于：

- 服务端：`cmd/paynetd/main.go`
- 客户端：`cmd/paynetcli/main.go`

下文给出最小可运行片段及步骤。

---

## 1. 准备工作

1. 生成或复用 TLS 证书
   ```bash
   scripts/gen_certs.sh paynet.local device-demo
   # 额外设备证书：scripts/gen_device_cert.sh device-002
   ```
2. 编译 Protobuf
   ```bash
   protoc --go_out=internal/pb --go_opt=paths=source_relative paybox.proto
   ```

---

## 2. 服务端示例

```go
package main

import (
    "context"
    "log"
    "time"

    payboxpb "github.com/wfunc/paynet/internal/pb"
    "github.com/wfunc/paynet/pkg/paynet"
    "google.golang.org/protobuf/proto"
)

func main() {
    // 注册消息与处理函数
    paynet.MustRegister(paynet.MessageMeta{
        Type: paynet.TypeRegisterReq,
        Name: "paybox.RegisterReq",
        Factory: func() proto.Message { return &payboxpb.RegisterReq{} },
        Handler: paynet.HandlerFunc(handleRegister),
    })
    paynet.MustRegister(paynet.MessageMeta{
        Type: paynet.TypePing,
        Name: "paybox.Ping",
        Factory: func() proto.Message { return &payboxpb.Ping{} },
        Handler: paynet.HandlerFunc(handlePing),
    })
    // 其他消息同理

    srv, err := paynet.NewServer(paynet.ServerConfig{
        Addr: ":9443",
        TLS: paynet.TLSOption{
            CertFile:          "./certs/server.crt",
            KeyFile:           "./certs/server.key",
            CAFile:            "./certs/ca.crt",
            RequireClientCert: true,
        },
        MaxFrame:    64 * 1024,
        ReadTimeout: 70 * time.Second,
    })
    if err != nil {
        log.Fatalf("server init: %v", err)
    }

    if err := srv.ListenAndServe(context.Background()); err != nil {
        log.Fatalf("server exit: %v", err)
    }
}

func handleRegister(ctx paynet.Context, msg proto.Message) error {
    req := msg.(*payboxpb.RegisterReq)
    rsp := &payboxpb.RegisterRsp{
        Code:     payboxpb.ResultCode_OK,
        DeviceId: "dev-" + req.DeviceSn,
        Msg:      "registered",
    }
    return ctx.Send(rsp, paynet.WithType(paynet.TypeRegisterRsp))
}

func handlePing(ctx paynet.Context, msg proto.Message) error {
    pong := &payboxpb.Pong{Timestamp: uint64(time.Now().UnixMilli())}
    return ctx.Send(pong, paynet.WithType(paynet.TypePong))
}
```

注意：若只需单向 TLS，将 `RequireClientCert` 设为 `false`，客户端亦无需 `CertFile/KeyFile`。

---

## 3. 客户端示例

```go
package main

import (
    "context"
    "log"
    "time"

    payboxpb "github.com/wfunc/paynet/internal/pb"
    "github.com/wfunc/paynet/pkg/paynet"
    "google.golang.org/protobuf/proto"
)

func main() {
    registerMessages()

    client, err := paynet.NewClient(paynet.ClientConfig{
        Addr: "paynet.local:9443",
        TLS: paynet.TLSOption{
            CertFile:   "./certs/device-demo.crt",
            KeyFile:    "./certs/device-demo.key",
            CAFile:     "./certs/ca.crt",
            ServerName: "paynet.local",
        },
        HeartbeatInterval: 60 * time.Second,
        HeartbeatFactory: func() proto.Message {
            return &payboxpb.Ping{Timestamp: uint64(time.Now().UnixMilli())}
        },
    })
    if err != nil {
        log.Fatalf("client init: %v", err)
    }

    client.Subscribe(paynet.TypeCoinCommand, paynet.HandlerFunc(func(ctx paynet.Context, m proto.Message) error {
        cmd := m.(*payboxpb.CoinCommand)
        log.Printf("command: %+v", cmd)
        ack := &payboxpb.CoinAck{OrderId: cmd.OrderId, State: payboxpb.CoinState_COIN_ACCEPTED}
        return client.Send(context.Background(), ack, paynet.WithType(paynet.TypeCoinAck))
    }))

    flow := paynet.LoginFlow{
        Register: &payboxpb.RegisterReq{DeviceSn: "SN-001", Model: "demo"},
        Login:    &payboxpb.LoginReq{DeviceId: "dev-SN-001", Token: "demo-token"},
    }
    if err := client.Run(context.Background(), flow); err != nil {
        log.Fatalf("client stopped: %v", err)
    }
}

func registerMessages() {
    paynet.MustRegister(paynet.MessageMeta{
        Type:    paynet.TypeRegisterRsp,
        Name:    "paybox.RegisterRsp",
        Factory: func() proto.Message { return &payboxpb.RegisterRsp{} },
    })
    paynet.MustRegister(paynet.MessageMeta{
        Type:    paynet.TypeCoinCommand,
        Name:    "paybox.CoinCommand",
        Factory: func() proto.Message { return &payboxpb.CoinCommand{} },
    })
    // 其他消息同理
}
```

---

## 4. 运行步骤

1. 启动服务端：
   ```bash
   go run ./cmd/paynetd
   ```
2. 启动客户端：
   ```bash
   go run ./cmd/paynetcli
   ```
3. 查看日志可见 Register → Login → Heartbeat → CoinCommand/Query 的往返。

---

## 5. 项目接入建议

1. 把 `pkg/paynet` 当作模块引入，业务项目只需：
   - 注册自定义 `.proto` 生成的消息与 Handler；
   - 在 Handler 内通过 `ctx.SetAttr/ctx.Attr` 维护会话状态，调用 `ctx.Send` 回复；
   - 使用 `paynet.Client`/`paynet.Server` 统一处理 TLS、心跳、重连、幂等。
2. 将 Type 映射集中管理（可复用 `pkg/paynet/message_ids.go` 或自定义）。
3. 若要多语言接入（C/嵌入式），复用同一份 `paybox.proto` 和帧格式即可，TLS 与消息调度由对应语言实现。
