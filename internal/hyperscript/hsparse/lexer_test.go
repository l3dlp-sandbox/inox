package hsparse

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLexer(t *testing.T) {

	t.Run("ok", func(t *testing.T) {
		const source = "on click toggle .red on me"
		res, _, _ := ParseHyperScriptSlow(source)

		lexer := NewLexer()
		tokens, err := lexer.tokenize(source, false)
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, res.Tokens, tokens)
	})

	t.Run("unknown token", func(t *testing.T) {
		const source = "on click toggle . on me"
		_, parsingErr, _ := ParseHyperScriptSlow(source)

		lexer := NewLexer()
		tokens, err := lexer.tokenize(source, false)
		if !assert.Error(t, err) {
			return
		}

		assert.Equal(t, parsingErr.Tokens, tokens)
	})
}
