package selector

import "github.com/kcmvp/xql/cmd/gob/xql/testhelpers"

type X struct{}

func (X) Table() string { return testhelpers.ExportedTable }
