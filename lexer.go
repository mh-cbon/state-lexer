// This package provides a Lexer that functions similarly to Rob Pike's discussion
// about lexer design in this [talk](https://www.youtube.com/watch?v=HxaD_trXwRE).
//
// Original implementation forked from https://github.com/bbuck/go-lexer.
//
// You can define your token types by using the `lexer.TokenType` type (`int`) via
//
//     const (
//             StringToken lexer.TokenType = iota
//             IntegerToken
//             // etc...
//     )
//
// And then you define your own state functions (`lexer.StateFunc`) to handle
// analyzing the string.
//
//     func StringState(l *lexer.L) lexer.StateFunc {
//             l.Next() // eat starting "
//             l.Ignore() // drop current value
//             while l.Peek() != '"' {
//                     l.Next()
//             }
//             l.Emit(StringToken)
//
//             return SomeStateFunction
//     }
//
package lexer

import (
	"errors"
	"io"
	"strings"
	"unicode/utf8"
)

type StateFunc func(*L) StateFunc

type TokenType int

const (
	EOFRune    rune      = -1
	EmptyToken TokenType = 0
)

type Token struct {
	Type  TokenType
	Value string
}

func (t *Token) GetType() TokenType {
	return t.Type
}
func (t *Token) GetValue() string {
	return t.Value
}
func (t *Token) String() string {
	return t.GetValue()
}

type L struct {
	source          io.Reader
	start, position int
	readbytes       int
	buf             []rune
	p               []byte
	startState      StateFunc
	Err             error
	// tokens          chan Token
	TokenHandler func(t Token)
	ErrorHandler func(e string)
	rewind       runeStack
	hasNext      bool
	nextState    StateFunc
	lastTokens   []*Token
}

// New creates a returns a lexer ready to parse the given source code.
func New(src io.Reader, start StateFunc) *L {
	return &L{
		source:     src,
		startState: start,
		buf:        make([]rune, 0),
		p:          make([]byte, 1),
		start:      0,
		position:   0,
		readbytes:  0,
		rewind:     newRuneStack(),
	}
}

//NextTokens Reads until at least one token is met, it returns nil a []*Token{nil} at EOF.
func (l *L) NextTokens() []*Token {
	if l.hasNext == false {
		l.TokenHandler = func(t Token) {
			l.lastTokens = append(l.lastTokens, &t)
		}
		l.lastTokens = l.lastTokens[:0]
		l.nextState = l.startState
		l.hasNext = true
	}
	state := l.nextState
	l.nextState = state(l)
	if l.nextState == nil {
		l.lastTokens = l.lastTokens[:0]
		l.nextState = l.startState
		l.hasNext = false
		return []*Token{nil}
	}
	ret := l.lastTokens[:]
	l.lastTokens = l.lastTokens[:0]
	return ret
}

//NextToken Reads until a token is met, it returns nil at EOF.
func (l *L) NextToken() *Token {
	var ret *Token
	if l.hasNext == false {
		l.TokenHandler = func(t Token) {
			l.lastTokens = append(l.lastTokens, &t)
		}
		l.lastTokens = l.lastTokens[:0]
		l.nextState = l.startState
		l.hasNext = true
	}
	if len(l.lastTokens) > 0 {
		ret = l.lastTokens[0]
		l.lastTokens = l.lastTokens[1:]
	} else {
		for l.nextState != nil {
			state := l.nextState
			l.nextState = state(l)
			if len(l.lastTokens) > 0 {
				break
			}
		}
		if l.nextState == nil {
			l.lastTokens = append(l.lastTokens, nil)
		}
		if len(l.lastTokens) > 0 {
			ret = l.lastTokens[0]
			l.lastTokens = l.lastTokens[1:]
		}
	}
	return ret
}

//Scan Broweses all tokens and invokdes f for each of them.
func (l *L) Scan(f func(t Token)) {
	l.TokenHandler = f
	state := l.startState
	for state != nil {
		state = state(l)
	}
}

// Not Helper function
func Not(t TokenType, f func(Token)) func(Token) {
	return func(token Token) {
		if token.Type != t {
			f(token)
		}
	}
}

// Current returns the value being analyzed at this moment.
func (l *L) Current() string {
	return string(l.buf[l.start:l.position])
}

// Emit will receive a token type and push a new token with the current analyzed
// value into the tokens channel.
func (l *L) Emit(t TokenType) {
	tok := Token{
		Type:  t,
		Value: l.Current(),
	}
	if l.TokenHandler != nil {
		l.TokenHandler(tok)
	}
	// l.tokens <- tok
	l.buf = l.buf[l.position:]
	l.start = 0
	l.position = 0
	l.rewind.clear()
}

// Ignore clears the rewind stack and then sets the current beginning position
// to the current position in the source which effectively ignores the section
// of the source being analyzed.
func (l *L) Ignore() {
	l.rewind.clear()
	l.buf = l.buf[l.position:]
	l.start = 0
	l.position = 0
}

// ReadBytes returns number of byte reead.
func (l *L) ReadBytes() int {
	return l.readbytes
}

// Peek performs a Next operation immediately followed by a Rewind returning the
// peeked rune.
func (l *L) Peek() rune {
	r := l.Next()
	l.Rewind()
	return r
}

// Rewind will take the last rune read (if any) and rewind back. Rewinds can
// occur more than once per call to Next but you can never rewind past the
// last point a token was emitted.
func (l *L) Rewind() {
	r := l.rewind.pop()
	if r > EOFRune {
		size := utf8.RuneLen(r)
		l.position -= size
		if l.position < l.start {
			l.position = l.start
		}
	}
}

// Next pulls the next rune from the Lexer and returns it, moving the position
// forward in the source.
func (l *L) Next() rune {
	var (
		r rune
		s int
	)
	if l.position < len(l.buf) {
		r = l.buf[l.position:][0]
		l.position += utf8.RuneLen(r)
		l.rewind.push(r)
		return r
	}

	n, _ := l.source.Read(l.p)
	l.readbytes += n
	if n == 0 {
		r, s = EOFRune, 0
	} else {
		r, s = utf8.DecodeRune(l.p)
		l.buf = append(l.buf, r)
	}
	l.position += s
	l.rewind.push(r)

	return r
}

// Take receives a string containing all acceptable strings and will contine
// over each consecutive character in the source until a token not in the given
// string is encountered. This should be used to quickly pull token parts.
func (l *L) Take(chars string) {
	r := l.Next()
	for strings.ContainsRune(chars, r) {
		r = l.Next()
	}
	l.Rewind() // last next wasn't a match
}

func (l *L) Error(e string) {
	if l.ErrorHandler != nil {
		l.Err = errors.New(e)
		l.ErrorHandler(e)
	} else {
		panic(e)
	}
}

// // Private methods
func (l *L) scanOnce(f func(t Token)) {
	l.TokenHandler = f
	if l.startState != nil {
		l.startState = l.startState(l)
	}
}
