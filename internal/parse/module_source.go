package internal

import (
	"bytes"
	"fmt"
	"io"
)

type ParsedChunk struct {
	Node   *Chunk
	Source ChunkSource
	runes  []rune
}

func (c ParsedChunk) Name() string {
	return c.Source.Name()
}

func (c *ParsedChunk) getRunes() []rune {
	runes := c.runes
	if c.Source.Code() != "" && len(runes) == 0 {
		c.runes = []rune(c.Source.Code())
	}
	return runes
}

type ChunkSource interface {
	Name() string
	Code() string
}

type SourceFile struct {
	NameString    string
	Resource      string //path or url
	ResourceDir   string //path or url
	IsResourceURL bool
	CodeString    string
}

func (f SourceFile) Name() string {
	return f.NameString
}

func (f SourceFile) Code() string {
	return f.CodeString
}

type InMemorySource struct {
	NameString string
	CodeString string
}

func (s InMemorySource) Name() string {
	return s.NameString
}

func (s InMemorySource) Code() string {
	return s.CodeString
}

func ParseChunkSource(src ChunkSource) (*ParsedChunk, error) {
	runes, chunk, err := ParseChunk2(src.Code(), src.Name())

	if chunk == nil {
		return nil, err
	}

	return &ParsedChunk{
		Node:   chunk,
		Source: src,
		runes:  runes,
	}, err
}

func NewParsedChunk(node *Chunk, src ChunkSource) *ParsedChunk {
	return &ParsedChunk{
		Node:   node,
		Source: src,
	}
}

func (chunk *ParsedChunk) GetLineColumn(node Node) (int32, int32) {
	return chunk.GetSpanLineColumn(node.Base().Span)
}

func (chunk *ParsedChunk) FormatNodeLocation(w io.Writer, node Node) (int, error) {
	line, col := chunk.GetLineColumn(node)
	return fmt.Fprintf(w, "%s:%d:%d:", chunk.Name(), line, col)
}

func (chunk *ParsedChunk) GetFormattedNodeLocation(node Node) string {
	buf := bytes.NewBuffer(nil)
	chunk.FormatNodeLocation(buf, node)
	return buf.String()
}

func (chunk *ParsedChunk) GetSpanLineColumn(span NodeSpan) (int32, int32) {
	line := int32(1)
	col := int32(1)
	i := 0

	runes := chunk.getRunes()

	for i < int(span.Start) && i < len(runes) {
		if runes[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}

		i++
	}

	return line, col
}

func (chunk *ParsedChunk) GetLineColumnSingeCharSpan(line, column int32) NodeSpan {
	i := int32(0)
	runes := chunk.getRunes()
	length := len32(runes)

	line -= 1

	for i < length && line > 0 {
		if runes[i] == '\n' {
			line--
		}
		i++
	}

	pos := i + column

	return NodeSpan{
		Start: pos,
		End:   pos + 1,
	}
}

func (chunk *ParsedChunk) GetSourcePosition(span NodeSpan) SourcePosition {
	l, c := chunk.GetSpanLineColumn(span)
	return SourcePosition{SourceName: chunk.Name(), Line: l, Column: c, Span: span}
}

func (chunk *ParsedChunk) GetNodeAtSpan(target NodeSpan) (foundNode Node, ok bool) {

	Walk(chunk.Node, func(node, _, _ Node, _ []Node, _ bool) (TraversalAction, error) {
		span := node.Base().Span

		//if the cursor is not in the node's span we don't check the descendants of the node
		if span.Start > target.End || span.End < target.Start {
			return Prune, nil
		}

		if foundNode == nil || node.Base().IncludedIn(foundNode) {
			foundNode = node
			ok = true
		}

		return Continue, nil
	}, nil)

	return
}

type SourcePosition struct {
	SourceName string   `json:"sourceName"`
	Line       int32    `json:"line"`
	Column     int32    `json:"column"`
	Span       NodeSpan `json:"span"`
}

func (pos SourcePosition) String() string {
	return fmt.Sprintf("%s:%d:%d:", pos.SourceName, pos.Line, pos.Column)
}

type SourcePositionStack []SourcePosition

func (stack SourcePositionStack) String() string {
	buff := bytes.NewBuffer(nil)
	for _, pos := range stack {
		buff.WriteString(pos.String())
		buff.WriteRune(' ')
	}
	return buff.String()
}
