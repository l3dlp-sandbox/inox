package css

import (
	"fmt"

	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/css"
)

func ParseString(s string) (Node, error) {
	input := parse.NewInputString(s)
	parser := css.NewParser(input, false)

	var stack []Node

	stack = append(stack, Node{
		Type: Stylesheet,
	})

	current := 0
	parent := -1

	for {
		gt, _, data := parser.Next()

		switch gt {
		case css.CommentGrammar:
			comment := Node{
				Type: Comment,
				Data: string(data),
			}
			stack[current].Children = append(stack[current].Children, comment)
		case css.BeginAtRuleGrammar:
			stack = append(stack, Node{Type: AtRule})
			parent++
			current++

			stack[current].Children = append(stack[current].Children, Node{
				Type: MediaQuery,
			})

			mediaQuery := &stack[current].Children[0]

			err := makeNodesFromTokens(parser.Values(), mediaQuery)
			if err != nil {
				return Node{}, err
			}
		case css.BeginRulesetGrammar:
			stack = append(stack, Node{Type: Ruleset})
			current++
			parent++

			stack[current].Children = append(stack[current].Children, Node{
				Type: Selector,
			})

			selector := &stack[current].Children[0]

			err := makeNodesFromTokens(parser.Values(), selector)
			if err != nil {
				return Node{}, err
			}
		case css.DeclarationGrammar:
			stack[current].Children = append(stack[current].Children, Node{
				Type: Declaration,
			})

			declaration := &stack[current].Children[len(stack[current].Children)-1]
			declaration.Data = string(data)

			err := makeNodesFromTokens(parser.Values(), declaration)
			if err != nil {
				return Node{}, err
			}
		case css.EndAtRuleGrammar, css.EndRulesetGrammar:
			prev := stack[current]
			parent--
			current--
			stack = stack[:len(stack)-1]

			stack[current].Children = append(stack[current].Children, prev)
		}

		// fmt.Println(" <" + gt.String() + "> ")

		// for _, val := range parser.Values() {
		// 	fmt.Println(" (" + val.String() + ") ")
		// }

		if gt == css.ErrorGrammar {
			break
		}
	}

	return stack[0], nil
}

func makeNodesFromTokens(tokens []css.Token, parentNode *Node) error {
	precededByDot := false
	leadingSpace := true

	for _, t := range tokens {
		if t.TokenType == css.WhitespaceToken && leadingSpace {
			continue
		}

		leadingSpace = false

		if t.TokenType == css.DelimToken {
			precededByDot = true
			continue
		}

		if precededByDot && t.TokenType != css.IdentToken {
			return fmt.Errorf("delim '.' not followed by an identifier")
		}

		node, isSignificant := makeNodeFromToken(t, precededByDot)
		if isSignificant {
			parentNode.Children = append(parentNode.Children, node)
		}
		precededByDot = false
	}
	return nil
}

func makeNodeFromToken(t css.Token, precededByDot bool) (n Node, significant bool) {

	if precededByDot && t.TokenType != css.IdentToken {
		panic(fmt.Errorf("onlt identifiers can be preceded by a dot"))
	}

	switch t.TokenType {
	case css.NumberToken:
		n.Type = Number
	case css.DimensionToken:
		n.Type = Dimension
	case css.IdentToken:
		if precededByDot {
			n.Type = ClassName
		} else {
			n.Type = Ident
		}
	case css.FunctionToken:
		n.Type = Function
	case css.HashToken:
		n.Type = Hash
	case css.StringToken:
		n.Type = String
	case css.BadStringToken:
		n.Type = String
		n.Error = true
	case css.URLToken:
		n.Type = URL
	case css.BadURLToken:
		n.Type = URL
		n.Error = true
	case css.PercentageToken:
		n.Type = Percentage
	case css.UnicodeRangeToken:
		n.Type = UnicodeRange
	case css.IncludeMatchToken, css.DashMatchToken, css.SuffixMatchToken, css.SubstringMatchToken:
		n.Type = MatchOperator
	case css.CustomPropertyNameToken:
		n.Type = CustomPropertyName
	case css.CustomPropertyValueToken:
		n.Type = CustomPropertyName
	case css.WhitespaceToken:
		n.Type = Whitespace
	default:
		return Node{}, false
	}

	n.Data = string(t.Data)
	significant = true
	return
}