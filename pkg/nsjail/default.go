package nsjail

import "google.golang.org/protobuf/proto"

var defaultConfig = &NsJailConfig{
	Mode:           Mode_ONCE.Enum(),
	Name:           proto.String("demo"),
	Hostname:       proto.String("sandbox"),
	TimeLimit:      proto.Uint32(60),
	MountProc:      proto.Bool(true),
	DetectCgroupv2: proto.Bool(true),
	IfaceNoLo:      proto.Bool(true),
	RlimitNproc:    proto.Uint64(5),
	CgroupMemMax:   proto.Uint64(1024 * 1024 * 100), // 100 MB
	CgroupPidsMax:  proto.Uint64(10),
	Mount: []*MountPt{
		{
			Src:    proto.String("/bin"),
			Dst:    proto.String("/bin"),
			IsBind: proto.Bool(true),
			Rw:     proto.Bool(false),
		},
		{
			Src:    proto.String("/lib"),
			Dst:    proto.String("/lib"),
			IsBind: proto.Bool(true),
			Rw:     proto.Bool(false),
		},
		{
			Src:    proto.String("/lib64"),
			Dst:    proto.String("/lib64"),
			IsBind: proto.Bool(true),
			Rw:     proto.Bool(false),
		},
		{
			Src:    proto.String("/usr/bin"),
			Dst:    proto.String("/usr/bin"),
			IsBind: proto.Bool(true),
			Rw:     proto.Bool(false),
		},
		{
			Src:    proto.String("/usr/lib"),
			Dst:    proto.String("/usr/lib"),
			IsBind: proto.Bool(true),
			Rw:     proto.Bool(false),
		},
		{
			Src:    proto.String("/usr/lib64"),
			Dst:    proto.String("/usr/lib64"),
			IsBind: proto.Bool(true),
			Rw:     proto.Bool(false),
		},
		{
			Src:    proto.String("/dev/null"),
			Dst:    proto.String("/dev/null"),
			IsBind: proto.Bool(true),
			Rw:     proto.Bool(true),
		},
		{
			Src:    proto.String("/dev/urandom"),
			Dst:    proto.String("/dev/urandom"),
			IsBind: proto.Bool(true),
			Rw:     proto.Bool(true),
		},
		{
			Src:    proto.String("/dev/random"),
			Dst:    proto.String("/dev/random"),
			IsBind: proto.Bool(true),
			Rw:     proto.Bool(true),
		},
		{
			Src:    proto.String("/tmp"),
			Dst:    proto.String("/tmp"),
			IsBind: proto.Bool(true),
			Rw:     proto.Bool(true),
		},
	},
}

func DefaultConfig() *NsJailConfig {
	return proto.Clone(defaultConfig).(*NsJailConfig)
}
