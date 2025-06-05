package main

import (
	"fmt"
	"os"

	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"

	nsjailpb "github.com/Mirai3103/remote-compiler/pkg/nsjail"
)

func main() {
	cfg := &nsjailpb.NsJailConfig{
		Name:      proto.String("demo"),
		Hostname:  proto.String("sandbox"),
		Mode:      nsjailpb.Mode_ONCE.Enum(),
		TimeLimit: proto.Uint32(60),
		MountProc: proto.Bool(true),
		ExecBin: &nsjailpb.Exe{
			Path: proto.String("/bin/echo"),
			Arg:  []string{"hello from jail"},
		},
	}

	// Serialize sang textproto format
	out, err := prototext.MarshalOptions{Multiline: true}.Marshal(cfg)
	if err != nil {
		panic(err)
	}

	if err := os.WriteFile("demo_config.textproto", out, 0644); err != nil {
		panic(err)
	}

	fmt.Println("Config saved to demo_config.textproto")
}
