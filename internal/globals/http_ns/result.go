package http_ns

import (
	"fmt"
	"net/http"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	html_ns_symb "github.com/inoxlang/inox/internal/globals/html_ns/symbolic"
	http_ns_symb "github.com/inoxlang/inox/internal/globals/http_ns/symbolic"
)

const (
	RESULT_INIT_STATUS_PROPNAME  = "status"
	RESULT_INIT_BODY_PROPNAME    = "body"
	RESULT_INIT_HEADERS_PROPNAME = "headers"
)

var (
	SYMBOLIC_RESULT_INIT_ARG = symbolic.NewInexactObject(
		map[string]symbolic.Serializable{
			RESULT_INIT_STATUS_PROPNAME:  http_ns_symb.ANY_STATUS_CODE,
			RESULT_INIT_BODY_PROPNAME:    symbolic.AsSerializableChecked(symbolic.NewMultivalue(html_ns_symb.ANY_HTML_NODE, symbolic.ANY_STR_LIKE)),
			RESULT_INIT_HEADERS_PROPNAME: symbolic.NewInexactObject2(map[string]symbolic.Serializable{}),
		},
		map[string]struct{}{RESULT_INIT_STATUS_PROPNAME: {}, RESULT_INIT_BODY_PROPNAME: {}, RESULT_INIT_HEADERS_PROPNAME: {}},
		nil)
	NEW_RESULT_PARAMS      = &[]symbolic.Value{SYMBOLIC_RESULT_INIT_ARG}
	NEW_RESULT_PARAM_NAMES = []string{"init"}

	_ = core.Value((*HttpResult)(nil))
)

type HttpResult struct {
	value   core.Serializable
	status  StatusCode
	headers http.Header
	//cookies []core.Serializable
}

func NewResult(ctx *core.Context, init *core.Object) *HttpResult {
	status := StatusCode(http.StatusOK)
	var value core.Serializable
	var headers http.Header

	init.ForEachEntry(func(k string, v core.Serializable) error {
		switch k {
		case RESULT_INIT_STATUS_PROPNAME:
			status = v.(StatusCode)
		case RESULT_INIT_BODY_PROPNAME:
			value = v
		case RESULT_INIT_HEADERS_PROPNAME:
			headers = http.Header{}
			v.(*core.Object).ForEachEntry(func(headerName string, headerValue core.Serializable) error {
				headers.Add(headerName, headerValue.(core.StringLike).GetOrBuildString())
				return nil
			})
		}
		return nil
	})

	return &HttpResult{
		value:   value,
		status:  status,
		headers: headers,
	}
}

func symbolicNewResult(ctx *symbolic.Context, init *symbolic.Object) *http_ns_symb.HttpResult {
	ctx.SetSymbolicGoFunctionParameters(NEW_RESULT_PARAMS, NEW_RESULT_PARAM_NAMES)

	if symbolic.HasRequiredOrOptionalProperty(init, RESULT_INIT_HEADERS_PROPNAME) {
		headers, ok := init.Prop(RESULT_INIT_HEADERS_PROPNAME).(*symbolic.Object)
		if ok {
			headers.ForEachEntry(func(headerName string, headerValue symbolic.Value) error {
				_, ok := headerValue.(symbolic.StringLike)
				if !ok {
					ctx.AddSymbolicGoFunctionError(fmt.Sprintf("invalid value for header %q, only string-like values are allowed", headerName))
				}
				return nil
			})
		}
	}

	return http_ns_symb.ANY_RESULT
}