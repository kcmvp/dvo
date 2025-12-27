package concat

const prefix = "pre_"

type X struct{}

func (X) Table() string { return prefix + "table" }
