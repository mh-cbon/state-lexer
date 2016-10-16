package lexer

import (
	"bytes"
	"fmt"
	"testing"
)

const (
	NumberToken TokenType = iota
	OpToken
	IdentToken
)

func NumberState(l *L) StateFunc {
	l.Take("0123456789")
	l.Emit(NumberToken)
	if l.Peek() == '.' {
		l.Next()
		l.Emit(OpToken)
		return IdentState
	}

	return nil
}

func IdentState(l *L) StateFunc {
	r := l.Next()
	for (r >= 'a' && r <= 'z') || r == '_' {
		r = l.Next()
	}
	l.Rewind()
	l.Emit(IdentToken)

	return WhitespaceState
}

func WhitespaceState(l *L) StateFunc {
	r := l.Next()
	if r == EOFRune {
		return nil
	}

	if r != ' ' && r != '\t' && r != '\n' && r != '\r' {
		l.Error(fmt.Sprintf("unexpected token %q", r))
		return nil
	}

	l.Take(" \t\n\r")
	l.Ignore()

	return NumberState
}

func Test_LexerMovingThroughString(t *testing.T) {
	b := bytes.NewBufferString("123")
	l := New(b, nil)
	run := []struct {
		s string
		r rune
	}{
		{"1", '1'},
		{"12", '2'},
		{"123", '3'},
		{"123", EOFRune},
	}

	for _, test := range run {
		r := l.Next()
		if r != test.r {
			t.Errorf("Expected %q but got %q", test.r, r)
			return
		}

		if l.Current() != test.s {
			t.Errorf("Expected %q but got %q", test.s, l.Current())
			return
		}
	}
}

func Test_LexingNumbers(t *testing.T) {
	b := bytes.NewBufferString("123")
	l := New(b, NumberState)
	var token *Token
	l.scanOnce(func(tok Token) {
		token = &tok
	})

	if token.Type != NumberToken {
		t.Errorf("Expected a %v but got %v", NumberToken, token.Type)
		return
	}

	if token.Value != "123" {
		t.Errorf("Expected %q but got %q", "123", token.Value)
		return
	}

	token = nil
	l.scanOnce(func(tok Token) {
		token = &tok
	})
	if token != nil {
		t.Error("Expected a nil token, but got %v", *token)
		return
	}
}

func Test_LexerRewind(t *testing.T) {
	b := bytes.NewBufferString("1")
	l := New(b, nil)

	r := l.Next()
	if r != '1' {
		t.Errorf("Expected %q but got %q", '1', r)
		return
	}

	if l.Current() != "1" {
		t.Errorf("Expected %q but got %q", "1", l.Current())
		return
	}

	l.Rewind()
	if l.Current() != "" {
		t.Errorf("Expected empty string, but got %q", l.Current())
		return
	}
}

func Test_MultipleTokens(t *testing.T) {
	cases := []struct {
		tokType TokenType
		val     string
	}{
		{NumberToken, "123"},
		{OpToken, "."},
		{IdentToken, "hello"},
		{NumberToken, "675"},
		{OpToken, "."},
		{IdentToken, "world"},
	}

	b := bytes.NewBufferString("123.hello  675.world")
	l := New(b, NumberState)

	var tokens []Token
	l.Scan(func(tok Token) {
		tokens = append(tokens, tok)
	})

	for i, c := range cases {
		if c.tokType != tokens[i].Type {
			t.Errorf("Expected token type %v but got %v", c.tokType, tokens[i].Type)
			return
		}

		if c.val != tokens[i].Value {
			t.Errorf("Expected %q but got %q", c.val, tokens[i].Value)
			return
		}
	}
}

func Test_LexerError(t *testing.T) {
	b := bytes.NewBufferString("1")
	l := New(b, WhitespaceState)
	l.ErrorHandler = func(e string) {}

	var token *Token
	l.Scan(func(tok Token) {
		token = &tok
	})

	if token != nil {
		t.Errorf("Expected no token, but got %v", *token)
		return
	}

	if l.Err == nil {
		t.Error("Expected an error to be on the lexer, but none found.")
		return
	}

	if l.Err.Error() != "unexpected token '1'" {
		t.Errorf("Expected specific message from error, but got %q", l.Err.Error())
		return
	}
}

func Example_Lexer() {
	b := bytes.NewBufferString("1 2 ")
	l := New(b, NumberState)
	l.ErrorHandler = func(e string) {}

	var tokens []Token
	l.Scan(func(tok Token) {
		tokens = append(tokens, tok)
	})

	fmt.Printf("%#v", tokens)
	//Output:
	//[]lexer.Token{lexer.Token{Type:0, Value:"1"}}
}
