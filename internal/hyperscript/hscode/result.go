package hscode

type ParsingResult struct {
	Node               Node    `json:"node"`
	Tokens             []Token `json:"tokens"`
	TokensNoWhitespace []Token `json:"tokensNoWhitespace"`
}

type ParsingError struct {
	Message        string  `json:"message"`
	MessageAtToken string  `json:"messageAtToken"`
	Token          Token   `json:"token"`
	Tokens         []Token `json:"tokens"`
}

func (e ParsingError) Error() string {
	return e.Message
}