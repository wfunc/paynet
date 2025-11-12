package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	payboxpb "github.com/wfunc/paynet/internal/pb"
	"github.com/wfunc/paynet/pkg/paynet"
	"google.golang.org/protobuf/proto"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	registerMessages()

	start := time.Now()
	client, err := paynet.NewClient(paynet.ClientConfig{
		Addr: "127.0.0.1:9443",
		TLS: paynet.TLSOption{
			CertFile:   "./certs/device.crt",
			KeyFile:    "./certs/device.key",
			CAFile:     "./certs/ca.crt",
			ServerName: "paynet.local",
		},
		HeartbeatInterval: 60 * time.Second,
		HeartbeatFactory: func() proto.Message {
			return &payboxpb.Ping{
				Timestamp: uint64(time.Now().UnixMilli()),
				Uptime:    uint32(time.Since(start).Seconds()),
			}
		},
	})
	if err != nil {
		log.Fatalf("init client: %v", err)
	}

	client.Subscribe(paynet.TypeCoinCommand, paynet.HandlerFunc(handleCoinCommand(client)))
	client.Subscribe(paynet.TypeQueryRsp, paynet.HandlerFunc(handleQueryRsp))

	flow := paynet.LoginFlow{
		Register: &payboxpb.RegisterReq{
			DeviceSn:     "SN-demo-001",
			Model:        "paybox-mini",
			Firmware:     "1.0.0",
			DevicePubkey: []byte("fake"),
		},
		Login: &payboxpb.LoginReq{
			DeviceId: "dev-SN-demo-001",
			Token:    "demo-token",
			Version:  "1.0.0",
		},
	}
	log.Println("[CLIENT] starting connection...")
	if err := client.Run(ctx, flow); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("client stopped: %v", err)
	}
}

func handleCoinCommand(cli *paynet.Client) func(paynet.Context, proto.Message) error {
	return func(ctx paynet.Context, msg proto.Message) error {
		cmd := msg.(*payboxpb.CoinCommand)
		log.Printf("[CLIENT] <- CoinCommand payload=%+v", cmd)
		ack := &payboxpb.CoinAck{
			OrderId:  cmd.OrderId,
			State:    payboxpb.CoinState_COIN_ACCEPTED,
			Progress: 0,
		}
		log.Printf("[CLIENT] -> CoinAck (ACCEPTED) payload=%+v", ack)
		if err := cli.Send(context.Background(), ack, paynet.WithType(paynet.TypeCoinAck)); err != nil {
			return err
		}
		go func() {
			time.Sleep(2 * time.Second)
			final := &payboxpb.CoinAck{
				OrderId:  cmd.OrderId,
				State:    payboxpb.CoinState_COIN_DONE,
				Progress: 100,
			}
			log.Printf("[CLIENT] -> CoinAck (DONE) payload=%+v", final)
			_ = cli.Send(context.Background(), final, paynet.WithType(paynet.TypeCoinAck))
		}()
		return nil
	}
}

func handleQueryRsp(_ paynet.Context, msg proto.Message) error {
	rsp := msg.(*payboxpb.QueryRsp)
	log.Printf("[CLIENT] <- QueryRsp payload=%+v", rsp)
	return nil
}

func registerMessages() {
	paynet.MustRegister(paynet.MessageMeta{
		Type:    paynet.TypeRegisterReq,
		Name:    "paybox.RegisterReq",
		Factory: func() proto.Message { return &payboxpb.RegisterReq{} },
	})
	paynet.MustRegister(paynet.MessageMeta{
		Type:    paynet.TypeRegisterRsp,
		Name:    "paybox.RegisterRsp",
		Factory: func() proto.Message { return &payboxpb.RegisterRsp{} },
		Handler: paynet.HandlerFunc(func(ctx paynet.Context, msg proto.Message) error {
			log.Printf("[CLIENT] <- RegisterRsp payload=%+v", msg)
			return nil
		}),
	})
	paynet.MustRegister(paynet.MessageMeta{
		Type:    paynet.TypeLoginReq,
		Name:    "paybox.LoginReq",
		Factory: func() proto.Message { return &payboxpb.LoginReq{} },
	})
	paynet.MustRegister(paynet.MessageMeta{
		Type:    paynet.TypeLoginRsp,
		Name:    "paybox.LoginRsp",
		Factory: func() proto.Message { return &payboxpb.LoginRsp{} },
		Handler: paynet.HandlerFunc(func(ctx paynet.Context, msg proto.Message) error {
			log.Printf("[CLIENT] <- LoginRsp payload=%+v", msg)
			return nil
		}),
	})
	paynet.MustRegister(paynet.MessageMeta{
		Type:    paynet.TypePing,
		Name:    "paybox.Ping",
		Factory: func() proto.Message { return &payboxpb.Ping{} },
	})
	paynet.MustRegister(paynet.MessageMeta{
		Type:    paynet.TypePong,
		Name:    "paybox.Pong",
		Factory: func() proto.Message { return &payboxpb.Pong{} },
		Handler: paynet.HandlerFunc(func(ctx paynet.Context, msg proto.Message) error {
			log.Printf("[CLIENT] <- Pong payload=%+v", msg)
			return nil
		}),
	})
	paynet.MustRegister(paynet.MessageMeta{
		Type:    paynet.TypeCoinCommand,
		Name:    "paybox.CoinCommand",
		Factory: func() proto.Message { return &payboxpb.CoinCommand{} },
	})
	paynet.MustRegister(paynet.MessageMeta{
		Type:    paynet.TypeCoinAck,
		Name:    "paybox.CoinAck",
		Factory: func() proto.Message { return &payboxpb.CoinAck{} },
	})
	paynet.MustRegister(paynet.MessageMeta{
		Type:    paynet.TypeQueryReq,
		Name:    "paybox.QueryReq",
		Factory: func() proto.Message { return &payboxpb.QueryReq{} },
	})
	paynet.MustRegister(paynet.MessageMeta{
		Type:    paynet.TypeQueryRsp,
		Name:    "paybox.QueryRsp",
		Factory: func() proto.Message { return &payboxpb.QueryRsp{} },
	})
	paynet.MustRegister(paynet.MessageMeta{
		Type:    paynet.TypeError,
		Name:    "paybox.ErrorMsg",
		Factory: func() proto.Message { return &payboxpb.ErrorMsg{} },
	})
}
