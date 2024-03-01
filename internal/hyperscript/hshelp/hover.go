package hshelp

import (
	"strings"

	"github.com/inoxlang/inox/internal/hyperscript/hscode"
)

func GetHoverHelpMarkdown(tokens []hscode.Token, cursorIndex int32) string {

	builder := strings.Builder{}

	token, ok := hscode.GetTokenAtCursor(cursorIndex, tokens)
	if ok {
		help, ok := HELP_DATA.ByTokenType[token.Type]
		if ok {
			builder.WriteString(help)
			builder.WriteByte('\n')
		}

		help, ok = HELP_DATA.ByTokenValue[token.Value]
		if ok {
			builder.WriteString(help)
			builder.WriteByte('\n')
		}
	}

	return builder.String()
}