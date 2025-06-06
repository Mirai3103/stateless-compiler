package main

import (
	"fmt"
	"os"

	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"

	nsjailpb "github.com/Mirai3103/remote-compiler/pkg/nsjail"
)

func main() {
	golangFibonacci := `
		package main
		import "fmt"
		func fib(n int) int {
			if n <= 0 {
				return 0
			} else if n == 1 {
				return 1
			} else {
				return fib(n-1) + fib(n-2)
			}
		}
		func main() {
			for i := 0; i < 10; i++ {
				fmt.Println(fib(i))
			}
		}
	`
	sourceFile := "./tmp/fib.go"
	if err := os.WriteFile(sourceFile, []byte(golangFibonacci), 0644); err != nil {
		panic(err)
	}
	sanboxDir := "/sandbox/box1"
	if err := os.MkdirAll(sanboxDir, 0755); err != nil {
		panic(err)
	}

	cfg := nsjailpb.DefaultConfig()
	mounts := cfg.GetMount()
	schootmounts := &nsjailpb.MountPt{
		Src:    proto.String(sanboxDir),
		Dst:    proto.String("/"),
		IsBind: proto.Bool(true),
		Rw:     proto.Bool(true),
	}
	sourceMount := &nsjailpb.MountPt{
		Src:    proto.String("./tmp/"),
		Dst:    proto.String("/app/"),
		IsBind: proto.Bool(true),
		Rw:     proto.Bool(true),
	}
	mounts = append(mounts, sourceMount)
	//  append first to mounts
	mounts = append([]*nsjailpb.MountPt{schootmounts}, mounts...)
	cfg.Mount = mounts
	cfg.StatsFile = proto.String("stats.txt")
	cfg.Cwd = proto.String("/app")

	cfg.ExecBin = &nsjailpb.Exe{
		Path: proto.String("/bin/bash"),
		Arg0: proto.String("/bin/bash"),
		Arg: []string{
			"-c",
			"go run /app/fib.go",
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
