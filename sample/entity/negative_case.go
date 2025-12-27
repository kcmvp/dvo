package entity

import "github.com/kcmvp/xql/entity"

type NegativeUnSupportTypeChannel struct {
	ChannelField chan int
}

type NegativeUnSupportTypeMap struct {
	MapField map[string]string
}

type NegativeUnSupportTypeSlice struct {
	SliceField []string
}

type NegativeUnSupportTypeFunc struct {
	FuncField func()
}

func (NegativeUnSupportTypeChannel) Table() string { return "negative_unsupported_type_channel" }
func (NegativeUnSupportTypeMap) Table() string     { return "negative_unsupported_type_map" }
func (NegativeUnSupportTypeSlice) Table() string   { return "negative_unsupported_type_slice" }
func (NegativeUnSupportTypeFunc) Table() string    { return "negative_unsupported_type_func" }

var (
	_ entity.Entity = (*NegativeUnSupportTypeChannel)(nil)
	_ entity.Entity = (*NegativeUnSupportTypeMap)(nil)
	_ entity.Entity = (*NegativeUnSupportTypeSlice)(nil)
	_ entity.Entity = (*NegativeUnSupportTypeFunc)(nil)
)
