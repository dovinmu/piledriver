package syntax

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/charmbracelet/lipgloss"
)

// Token represents a syntax-highlighted segment
type Token struct {
	Text     string
	Color    lipgloss.Color
	HasColor bool // true if Color is explicitly set
}

// Highlighter provides syntax highlighting for code
type Highlighter struct {
	cache map[string]chroma.Lexer
}

// New creates a new Highlighter
func New() *Highlighter {
	return &Highlighter{
		cache: make(map[string]chroma.Lexer),
	}
}

// Highlight tokenizes a line of code and returns colored tokens
// filename is used to detect the language
// If highlighting fails, returns a single token with the original text and empty color
func (h *Highlighter) Highlight(filename, line string) []Token {
	lexer := h.getLexer(filename)
	if lexer == nil {
		return []Token{{Text: line}}
	}

	iterator, err := lexer.Tokenise(nil, line)
	if err != nil {
		return []Token{{Text: line}}
	}

	var tokens []Token
	for _, token := range iterator.Tokens() {
		if token.Value == "" {
			continue
		}
		color, hasColor := tokenColor(token.Type)
		tokens = append(tokens, Token{
			Text:     token.Value,
			Color:    color,
			HasColor: hasColor,
		})
	}

	// If we got no tokens, return original
	if len(tokens) == 0 {
		return []Token{{Text: line}}
	}

	return tokens
}

// getLexer returns a lexer for the given filename, with caching
func (h *Highlighter) getLexer(filename string) chroma.Lexer {
	if lexer, ok := h.cache[filename]; ok {
		return lexer
	}

	lexer := lexers.Match(filename)
	if lexer == nil {
		return nil
	}

	// Use a coalesce config for better token grouping
	lexer = chroma.Coalesce(lexer)
	h.cache[filename] = lexer
	return lexer
}

// tokenColor maps chroma token types to terminal colors
// Using Dracula-inspired colors to match the existing theme
// Returns the color and whether a color was set
func tokenColor(t chroma.TokenType) (lipgloss.Color, bool) {
	switch {
	// Keywords: pink
	case t == chroma.Keyword,
		t == chroma.KeywordConstant,
		t == chroma.KeywordDeclaration,
		t == chroma.KeywordNamespace,
		t == chroma.KeywordPseudo,
		t == chroma.KeywordReserved,
		t == chroma.KeywordType:
		return lipgloss.Color("#ff79c6"), true

	// Strings: yellow
	case t == chroma.String,
		t == chroma.StringAffix,
		t == chroma.StringBacktick,
		t == chroma.StringChar,
		t == chroma.StringDelimiter,
		t == chroma.StringDoc,
		t == chroma.StringDouble,
		t == chroma.StringEscape,
		t == chroma.StringHeredoc,
		t == chroma.StringInterpol,
		t == chroma.StringOther,
		t == chroma.StringRegex,
		t == chroma.StringSingle,
		t == chroma.StringSymbol:
		return lipgloss.Color("#f1fa8c"), true

	// Numbers: purple
	case t == chroma.Number,
		t == chroma.NumberBin,
		t == chroma.NumberFloat,
		t == chroma.NumberHex,
		t == chroma.NumberInteger,
		t == chroma.NumberIntegerLong,
		t == chroma.NumberOct:
		return lipgloss.Color("#bd93f9"), true

	// Comments: lighter gray (distinct from context lines which use #6272a4)
	case t == chroma.Comment,
		t == chroma.CommentHashbang,
		t == chroma.CommentMultiline,
		t == chroma.CommentPreproc,
		t == chroma.CommentPreprocFile,
		t == chroma.CommentSingle,
		t == chroma.CommentSpecial:
		return lipgloss.Color("#9aabcf"), true

	// Functions/methods: cyan
	case t == chroma.NameFunction,
		t == chroma.NameFunctionMagic,
		t == chroma.NameBuiltin,
		t == chroma.NameBuiltinPseudo:
		return lipgloss.Color("#8be9fd"), true

	// Types/classes: cyan
	case t == chroma.NameClass,
		t == chroma.NameException:
		return lipgloss.Color("#8be9fd"), true

	// Operators: white (bright)
	case t == chroma.Operator,
		t == chroma.OperatorWord:
		return lipgloss.Color("#f8f8f2"), true

	// Punctuation: white
	case t == chroma.Punctuation:
		return lipgloss.Color("#f8f8f2"), true

	// Default: no special color, use base style
	default:
		return "", false
	}
}

// RenderLine renders a line with syntax highlighting, applying a base style
// The base style's foreground is used when token has no color
func RenderLine(tokens []Token, baseStyle lipgloss.Style) string {
	if len(tokens) == 0 {
		return ""
	}

	var b strings.Builder
	for _, tok := range tokens {
		if tok.HasColor {
			// Use syntax color
			style := lipgloss.NewStyle().Foreground(tok.Color)
			b.WriteString(style.Render(tok.Text))
		} else {
			// Use base style (diff color)
			b.WriteString(baseStyle.Render(tok.Text))
		}
	}

	return b.String()
}
