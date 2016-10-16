# state-lexer

This package provides a Lexer that functions similarly to Rob Pike's discussion
about lexer design in this [talk](https://www.youtube.com/watch?v=HxaD_trXwRE).

Original implementation forked from https://github.com/bbuck/go-lexer.

This fork remove uses of `chan lexer.Token` in flavor of a `func (t lexer.Token)` callback approach.

## Usage

You can define your token types by using the `lexer.TokenType` type (`int`) via

```go
const (
	StringToken lexer.TokenType = iota
	IntegerToken
	// etc...
)
```

And then you define your own state functions (`lexer.StateFunc`) to handle
analyzing the string.

```go
func StringState(l *lexer.L) lexer.StateFunc {
	l.Next() // eat starting "
	l.Ignore() // drop current value
	while l.Peek() != '"' {
		l.Next()
	}
	l.Emit(StringToken)

	return SomeStateFunction
}
```

Finally invoke a new instance of `Lexer.L` and call for `Scan()` method.

```go
package main

import (
	"bytes"
	"fmt"
	"github.com/mh-cbon/state-lexer"
)

const (
	NumberToken lexer.TokenType = iota
	WsToken
)

func isWhitespace(ch rune) bool { return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' }

func NumberState(l *lexer.L) lexer.StateFunc {
	// eat whitespace
	r := l.Next()
	for isWhitespace(r) {
		r = l.Next()
		if r == lexer.EOFRune {
			l.Emit(WsToken)
			return nil // signal end of parsing
		}
	}
	l.Rewind()      // put back last read, it is not a space
	l.Emit(WsToken) // emit WsToken for what we found

	l.Take("0123456789")
	l.Emit(NumberToken)

	return NumberState // signal next state
}

func main() {
	b := bytes.NewBufferString("1 2 ")
	l := lexer.New(b, NumberState)
	l.ErrorHandler = func(e string) {}

	var tokens []lexer.Token
	l.Scan(func(tok lexer.Token) {
		tokens = append(tokens, tok)
	})

	fmt.Printf("%#v", tokens)

	//Output:
	// []lexer.Token{
	//  lexer.Token{Type:1, Value:""},
	//  lexer.Token{Type:0, Value:"1"},
	//  lexer.Token{Type:1, Value:" "},
	//  lexer.Token{Type:0, Value:"2"},
	//  lexer.Token{Type:1, Value:" "},
	// }
}
```
