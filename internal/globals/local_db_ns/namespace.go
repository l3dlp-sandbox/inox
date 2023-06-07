package local_db_ns

import (
	core "github.com/inoxlang/inox/internal/core"
	symbolic "github.com/inoxlang/inox/internal/core/symbolic"
)

const (
	LDB_SCHEME = core.Scheme("ldb")
)

func init() {
	core.RegisterSymbolicGoFunction(openDatabase, func(ctx *symbolic.Context, r symbolic.ResourceName) (*SymbolicLocalDatabase, *symbolic.Error) {
		return &SymbolicLocalDatabase{}, nil
	})

	core.RegisterOpenDbFn(LDB_SCHEME, func(ctx *core.Context, resource core.SchemeHolder, resolutionData core.Value) (core.Database, error) {
		return openDatabase(ctx, resource)
	})
}

func NewLocalDbNamespace() *Record {
	return core.NewRecordFromMap(core.ValMap{
		//
		//"open": core.ValOf(openDatabase),
	})
}
