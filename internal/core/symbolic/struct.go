package symbolic

import pprint "github.com/inoxlang/inox/internal/prettyprint"

var (
	ANY_STRUCT_TYPE = &StructType{}
	ANY_STRUCT      = &Struct{typ: ANY_STRUCT_TYPE}

	_ = Value((*Struct)(nil))
)

// A Struct represents a symbolic Struct.
type Struct struct {
	typ *StructType
}

func (s *Struct) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherStruct, ok := v.(*Struct)
	if !ok {
		return false
	}
	return ok && s.typ.Equal(otherStruct.typ, state)
}

func (s *Struct) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("struct{")

	//TODO

	w.WriteByte('}')
}

func (s *Struct) WidestOfType() Value {
	return ANY_STRUCT
}

// StructType represents a struct type, it implements CompileTimeType.
type StructType struct {
	name    string
	fields  []structField //if nil any StructType is matched
	methods []structMethod
}

type structField struct {
	Name string
	Type CompileTimeType
}

type structMethod struct {
	Name string
	Type *InoxFunction
}

func (t *StructType) FieldCount() int {
	return len(t.fields)
}

func (t *StructType) Field(index int) structField {
	return t.fields[index]
}

func (t *StructType) Method(index int) structMethod {
	return t.methods[index]
}

func (t *StructType) MethodCount() int {
	return len(t.fields)
}

func (t *StructType) Equal(v CompileTimeType, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherStructType, ok := v.(*StructType)
	if !ok {
		return false
	}

	if t.fields == nil {
		return true
	}

	return otherStructType == t
}

func (t *StructType) TestValue(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	struct_, ok := v.(*Struct)
	if !ok {
		return false
	}
	return ok && struct_.typ == t
}

func (t *StructType) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("struct-type{")

	w.WriteString("...")
	w.WriteByte('}')
}