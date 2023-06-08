package local_db_ns

import (
	"bufio"

	core "github.com/inoxlang/inox/internal/core"
	symbolic "github.com/inoxlang/inox/internal/core/symbolic"
	pprint "github.com/inoxlang/inox/internal/pretty_print"

	"github.com/inoxlang/inox/internal/utils"
)

//

type SymbolicLocalDatabase struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (r *SymbolicLocalDatabase) Test(v SymbolicValue) bool {
	_, ok := v.(*SymbolicLocalDatabase)
	return ok
}

func (r SymbolicLocalDatabase) Clone(clones map[uintptr]SymbolicValue) symbolic.SymbolicValue {
	return &SymbolicLocalDatabase{}
}

func (r *SymbolicLocalDatabase) Widen() (symbolic.SymbolicValue, bool) {
	return nil, false
}

func (ldb *SymbolicLocalDatabase) Close() {

}

func (ldb *SymbolicLocalDatabase) Get(ctx *symbolic.Context, key *symbolic.Path) (SymbolicValue, *symbolic.Bool) {
	return &symbolic.Any{}, nil
}

func (ldb *SymbolicLocalDatabase) Has(ctx *symbolic.Context, key *symbolic.Path) *symbolic.Bool {
	return &symbolic.Bool{}
}

func (ldb *SymbolicLocalDatabase) Set(ctx *symbolic.Context, key *symbolic.Path, value SymbolicValue) {

}

func (ldb *SymbolicLocalDatabase) GetFullResourceName(pth Path) symbolic.ResourceName {
	return &symbolic.AnyResourceName{}
}

func (ldb *SymbolicLocalDatabase) Prop(name string) SymbolicValue {
	method, ok := ldb.GetGoMethod(name)
	if !ok {
		panic(symbolic.FormatErrPropertyDoesNotExist(name, ldb))
	}
	return method
}

func (ldb *SymbolicLocalDatabase) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	switch name {
	case "close":
		return symbolic.WrapGoMethod(ldb.Close), true
	}
	return nil, false
}

func (ldb *SymbolicLocalDatabase) PropertyNames() []string {
	return LOCAL_DB_PROPNAMES
}

func (a *SymbolicLocalDatabase) IsWidenable() bool {
	return false
}

func (r *SymbolicLocalDatabase) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%local-database")))
}

func (kvs *SymbolicLocalDatabase) WidestOfType() SymbolicValue {
	return &SymbolicLocalDatabase{}
}

///

func (kvs *LocalDatabase) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]SymbolicValue) (SymbolicValue, error) {
	return &SymbolicLocalDatabase{}, nil
}
