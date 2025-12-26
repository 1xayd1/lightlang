package main

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type Parser struct {
	input string
	pos   int
	line  int
}

func NewParser(input string) *Parser {
	return &Parser{input: input, pos: 0, line: 1}
}

func Parse(source string) ([]Node, error) {
	source = strings.ReplaceAll(source, "\r\n", "\n")
	source = strings.ReplaceAll(source, "\r", "\n")

	p := NewParser(source)
	return p.ParseProgram()
}

func (p *Parser) ParseProgram() ([]Node, error) {
	var nodes []Node
	for p.pos < len(p.input) {
		p.skipWhitespace()
		if p.pos >= len(p.input) {
			break
		}

		if p.matchKeyword("func") {
			p.pos += 4
			fnNode, err := p.parseFunctionDef()
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, fnNode)
			continue
		}
		if p.matchKeyword("if") {
			p.pos += 2 // <-- consume "if" critical bug number 99999
			ifNode, err := p.parseIfStatement()
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, ifNode)
			continue
		}
		if p.matchKeyword("while") {
			p.pos += 5
			whileNode, err := p.parseWhileLoop()
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, whileNode)
			continue
		}
		if p.matchKeyword("for") {
			p.pos += 3
			forNode, err := p.parseForLoop()
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, forNode)
			continue
		}
		if p.matchKeyword("let") {
			p.pos += 3
			stmt, err := p.parseLetAssignment()
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, stmt)
			p.consumeTerminator()
			continue
		}

		stmt, err := p.parseAssignmentOrExpr()
		if err != nil {
			return nil, err
		}
		if stmt != nil {
			nodes = append(nodes, stmt)
		}
		p.consumeTerminator()
	}
	return nodes, nil
}

func (p *Parser) parseLetAssignment() (Node, error) {
	p.skipWhitespace()
	start := p.pos
	for p.pos < len(p.input) && (unicode.IsLetter(rune(p.input[p.pos])) || unicode.IsDigit(rune(p.input[p.pos])) || p.input[p.pos] == '_') {
		p.pos++
	}
	if start == p.pos {
		return nil, fmt.Errorf("expected variable name after let")
	}
	varName := p.input[start:p.pos]
	p.skipWhitespace()
	if p.pos >= len(p.input) || p.input[p.pos] != '=' {
		return nil, fmt.Errorf("expected '=' in assignment")
	}
	p.pos++ // let the = DIE
	p.skipWhitespace()
	exprStr := p.readUntilTerminator()
	exprNode, err := parseExpression(exprStr)
	if err != nil {
		return nil, err
	}
	return &AssignmentNode{
		Name: varName,
		Expr: exprNode,
	}, nil
}

func (p *Parser) parseAssignmentOrExpr() (Node, error) {
	p.skipWhitespace()
	if p.pos >= len(p.input) {
		return nil, nil
	}

	start := p.pos
	inString := false
	escape := false
	parenDepth := 0

	for p.pos < len(p.input) {
		ch := p.input[p.pos]

		if escape {
			escape = false
			p.pos++
			continue
		}

		if ch == '\\' {
			escape = true
			p.pos++
			continue
		}

		if ch == '"' {
			inString = !inString
			p.pos++
			continue
		}

		if !inString {
			if ch == '(' {
				parenDepth++
			} else if ch == ')' {
				parenDepth--
			}

			if ch == ';' || ch == '\n' || ch == '\r' {
				break
			}

			if ch == '=' && parenDepth == 0 {
				if p.pos+1 < len(p.input) && p.input[p.pos+1] == '=' {
					p.pos += 2 // Skip '=='
					continue
				}
				if p.pos > 0 {
					prevChar := p.input[p.pos-1]
					if prevChar == '!' || prevChar == '<' || prevChar == '>' {
						p.pos++
						continue
					}
				}
				break
			}
		}
		p.pos++
	}

	leftStr := strings.TrimSpace(p.input[start:p.pos])

	if p.pos < len(p.input) && p.input[p.pos] == '=' && parenDepth == 0 {
		if p.pos > 0 && p.input[p.pos-1] == '=' {
		} else if p.pos > 0 && (p.input[p.pos-1] == '!' || p.input[p.pos-1] == '<' || p.input[p.pos-1] == '>') {
		} else {
			p.pos++
			p.skipWhitespace()
			rightStr := p.readUntilTerminator()

			// Handle index assignment
			if strings.Contains(leftStr, "[") && strings.Contains(leftStr, "]") {
				// ... index assignment logic
			}

			if !isVariable(leftStr) {
				return nil, fmt.Errorf("invalid left side of assignment: %s", leftStr)
			}
			valueNode, err := parseExpression(rightStr)
			if err != nil {
				return nil, err
			}

			return &AssignmentNode{
				Name: leftStr,
				Expr: valueNode,
			}, nil
		}
	}

	if leftStr == "" {
		return nil, nil
	}

	exprNode, err := parseExpression(leftStr)
	if err != nil {
		return nil, err
	}

	return &ExprStmtNode{Expr: exprNode}, nil
}

func (p *Parser) parseIfStatement() (Node, error) {
	// if* <cond> then* <body> [elseif <cond> then <body>] [else <body>]? end* * means the keyword is REQUIRED
	var conditions []Node
	var bodies [][]Node

	p.skipWhitespace()
	condStr := p.readUntilKeyword("then")
	condNode, err := parseExpression(condStr)
	if err != nil {
		return nil, err
	}
	conditions = append(conditions, condNode)

	body, err := p.parseBlockUntil([]string{"elseif", "else", "end"})
	if err != nil {
		return nil, err
	}
	bodies = append(bodies, body)

	for p.matchKeyword("elseif") {
		p.pos += 7
		p.skipWhitespace()
		condStr := p.readUntilKeyword("then")
		condNode, err := parseExpression(condStr)
		if err != nil {
			return nil, err
		}
		conditions = append(conditions, condNode)

		body, err := p.parseBlockUntil([]string{"elseif", "else", "end"})
		if err != nil {
			return nil, err
		}
		bodies = append(bodies, body)
	}

	var elseBody []Node
	if p.matchKeyword("else") {
		p.pos += 4
		var err error
		elseBody, err = p.parseBlockUntil([]string{"end"})
		if err != nil {
			return nil, err
		}
	}

	if !p.matchKeyword("end") {
		return nil, fmt.Errorf("expected 'end' to close if")
	}
	p.pos += 3
	p.consumeTerminator()

	return &IfNode{
		Conditions: conditions,
		Bodies:     bodies,
		ElseBody:   elseBody,
	}, nil
}

func (p *Parser) parseForLoop() (Node, error) {
	p.skipWhitespace()
	savedPos := p.pos

	loopType := ""

	tempPos := p.pos

	start := tempPos
	for tempPos < len(p.input) && (unicode.IsLetter(rune(p.input[tempPos])) || unicode.IsDigit(rune(p.input[tempPos])) || p.input[tempPos] == '_') {
		tempPos++
	}
	if start < tempPos {
		for tempPos < len(p.input) && (p.input[tempPos] == ' ' || p.input[tempPos] == '\t') {
			tempPos++
		}

		if tempPos < len(p.input) && p.input[tempPos] == '=' {
			tempPos++

			semicolonCount := 0
			inString := false
			for tempPos < len(p.input) {
				ch := p.input[tempPos]
				if ch == '"' {
					inString = !inString
				} else if !inString {
					if ch == ';' {
						semicolonCount++
						if semicolonCount == 2 {
							loopType = "cstyle"
							break
						}
					} else if p.matchKeywordAtPos("do", tempPos) {
						break
					}
				}
				tempPos++
			}
		}
	}

	p.pos = savedPos

	if loopType == "" {
		testStr := p.input[p.pos:]
		doPos := strings.Index(strings.ToLower(testStr), " do")
		if doPos > 0 {
			segment := testStr[:doPos]
			if strings.Contains(strings.ToLower(segment), " in ") {
				loopType = "in"
			}
		}
	}

	if loopType == "" {
		loopType = "cstyle"
	}

	switch loopType {
	case "cstyle":
		return p.parseCstyleForLoop()
	case "in":
		return p.parseInForLoop()
	default:
		return p.parseCstyleForLoop()
	}
}

func (p *Parser) parseCstyleForLoop() (Node, error) {
	p.skipWhitespace()

	hasParen := false
	if p.pos < len(p.input) && p.input[p.pos] == '(' {
		hasParen = true
		p.pos++
		p.skipWhitespace()
	}

	var initNode Node = nil
	if !p.matchKeyword(";") {
		var initStr string
		if hasParen {
			initStr = p.readUntil(";")
		} else {
			start := p.pos
			for p.pos < len(p.input) {
				if p.input[p.pos] == ';' {
					initStr = strings.TrimSpace(p.input[start:p.pos])
					p.pos++
					break
				}
				if p.matchKeywordAtPos("do", p.pos) {
					p.pos = start
					initStr = ""
					break
				}
				p.pos++
			}
		}

		if strings.TrimSpace(initStr) != "" {
			if strings.Contains(initStr, "=") && !strings.Contains(strings.ToLower(initStr), "let") {
				parts := strings.SplitN(initStr, "=", 2)
				if len(parts) == 2 {
					varName := strings.TrimSpace(parts[0])
					exprStr := strings.TrimSpace(parts[1])
					exprNode, err := parseExpression(exprStr)
					if err != nil {
						return nil, err
					}
					initNode = &AssignmentNode{
						Name: varName,
						Expr: exprNode,
					}
				}
			} else {
				var err error
				initNode, err = parseExpression(initStr)
				if err != nil {
					return nil, err
				}
			}
		}
	} else {
		p.pos++
	}

	p.skipWhitespace()

	var condNode Node = nil
	if !p.matchKeyword(";") {
		var condStr string
		if hasParen {
			condStr = p.readUntil(";")
		} else {
			start := p.pos
			for p.pos < len(p.input) {
				if p.input[p.pos] == ';' {
					condStr = strings.TrimSpace(p.input[start:p.pos])
					p.pos++
					break
				}
				if p.matchKeywordAtPos("do", p.pos) {
					p.pos = start
					condStr = ""
					break
				}
				p.pos++
			}
		}

		if strings.TrimSpace(condStr) != "" {
			var err error
			condNode, err = parseExpression(condStr)
			if err != nil {
				return nil, err
			}
		}
	} else {
		p.pos++
	}

	p.skipWhitespace()

	var updateNode Node = nil
	var updateStr string

	if hasParen {
		updateStr = p.readUntil(")")
		p.pos++
		p.skipWhitespace()
	} else {
		start := p.pos
		for p.pos < len(p.input) && !p.matchKeywordAtPos("do", p.pos) {
			p.pos++
		}
		if p.pos > start {
			updateStr = strings.TrimSpace(p.input[start:p.pos])
		}
	}

	if strings.TrimSpace(updateStr) != "" {
		if strings.Contains(updateStr, "=") {
			parts := strings.SplitN(updateStr, "=", 2)
			if len(parts) == 2 {
				varName := strings.TrimSpace(parts[0])
				exprStr := strings.TrimSpace(parts[1])
				exprNode, err := parseExpression(exprStr)
				if err != nil {
					return nil, err
				}
				updateNode = &AssignmentNode{
					Name: varName,
					Expr: exprNode,
				}
			}
		} else {
			var err error
			updateNode, err = parseExpression(updateStr)
			if err != nil {
				return nil, err
			}
		}
	}

	if !p.matchKeyword("do") {
		return nil, fmt.Errorf("expected 'do' after for loop condition")
	}
	p.pos += 2
	p.skipWhitespace()

	body, err := p.parseBlockUntil([]string{"end"})
	if err != nil {
		return nil, err
	}

	if !p.matchKeyword("end") {
		return nil, fmt.Errorf("expected 'end' for for loop")
	}
	p.pos += 3
	p.consumeTerminator()

	return &ForLoopNode{
		Init:   initNode,
		Cond:   condNode,
		Update: updateNode,
		Body:   body,
		Type:   "cstyle",
	}, nil
}

func (p *Parser) parseInForLoop() (Node, error) {
	p.skipWhitespace()

	start := p.pos
	for p.pos < len(p.input) && (unicode.IsLetter(rune(p.input[p.pos])) || unicode.IsDigit(rune(p.input[p.pos])) || p.input[p.pos] == '_') {
		p.pos++
	}
	if start == p.pos {
		return nil, fmt.Errorf("expected variable name in for loop")
	}
	loopVar := p.input[start:p.pos]

	p.skipWhitespace()

	if !p.matchKeyword("in") {
		return nil, fmt.Errorf("expected 'in' in for loop")
	}
	p.pos += 2

	p.skipWhitespace()

	startPos := p.pos
	for p.pos < len(p.input) && !p.matchKeywordAtPos("do", p.pos) {
		p.pos++
	}

	if p.pos >= len(p.input) {
		return nil, fmt.Errorf("expected 'do' after for loop collection")
	}

	collectionStr := strings.TrimSpace(p.input[startPos:p.pos])
	collectionNode, err := parseExpression(collectionStr)
	if err != nil {
		return nil, err
	}

	if !p.matchKeyword("do") {
		return nil, fmt.Errorf("expected 'do' after for loop collection")
	}
	p.pos += 2

	body, err := p.parseBlockUntil([]string{"end"})
	if err != nil {
		return nil, err
	}

	if !p.matchKeyword("end") {
		return nil, fmt.Errorf("expected 'end' for for loop")
	}
	p.pos += 3
	p.consumeTerminator()

	return &ForLoopNode{
		LoopVar:    loopVar,
		Collection: collectionNode,
		Body:       body,
		Type:       "in",
	}, nil
}

func (p *Parser) matchKeywordAtPos(kw string, pos int) bool {
	if pos+len(kw) > len(p.input) {
		return false
	}
	sub := p.input[pos : pos+len(kw)]
	if sub != kw {
		return false
	}
	nextIdx := pos + len(kw)
	if nextIdx >= len(p.input) {
		return true
	}
	next := rune(p.input[nextIdx])
	return !unicode.IsLetter(next) && !unicode.IsDigit(next) && next != '_'
}

func (p *Parser) readUntil(stopChar string) string {
	start := p.pos
	for p.pos < len(p.input) && string(p.input[p.pos]) != stopChar {
		p.pos++
	}
	result := p.input[start:p.pos]
	if p.pos < len(p.input) && string(p.input[p.pos]) == stopChar {
		p.pos++
	}
	return strings.TrimSpace(result)
}

func (p *Parser) parseWhileLoop() (Node, error) {
	p.skipWhitespace()
	condStr := p.readUntilKeyword("do")
	condNode, err := parseExpression(condStr)
	if err != nil {
		return nil, err
	}

	body, err := p.parseBlockUntil([]string{"end"})
	if err != nil {
		return nil, err
	}

	if !p.matchKeyword("end") {
		return nil, fmt.Errorf("expected 'end' for while loop")
	}
	p.pos += 3
	p.consumeTerminator()

	return &WhileLoopNode{Condition: condNode, Body: body}, nil
}

func (p *Parser) parseFunctionDef() (Node, error) {
	p.skipWhitespace()
	start := p.pos
	for p.pos < len(p.input) && (unicode.IsLetter(rune(p.input[p.pos])) || unicode.IsDigit(rune(p.input[p.pos])) || p.input[p.pos] == '_') {
		p.pos++
	}
	name := p.input[start:p.pos]
	p.skipWhitespace()

	if p.pos >= len(p.input) || p.input[p.pos] != '(' {
		return nil, fmt.Errorf("expect '(' in function definition")
	}
	p.pos++
	var params []string

	for {
		p.skipWhitespace()
		if p.pos >= len(p.input) {
			return nil, fmt.Errorf("unclosed parameters")
		}
		if p.input[p.pos] == ')' {
			p.pos++
			break
		}
		argStart := p.pos
		for p.pos < len(p.input) && (unicode.IsLetter(rune(p.input[p.pos])) || unicode.IsDigit(rune(p.input[p.pos])) || p.input[p.pos] == '_') {
			p.pos++
		}
		if argStart == p.pos {
			return nil, fmt.Errorf("expected parameter name")
		}
		params = append(params, p.input[argStart:p.pos])
		p.skipWhitespace()
		if p.pos < len(p.input) {
			if p.input[p.pos] == ',' {
				p.pos++
				continue
			} else if p.input[p.pos] == ')' {
				p.pos++
				break
			}
		}
	}

	body, err := p.parseBlockUntil([]string{"end"})
	if err != nil {
		return nil, err
	}

	if !p.matchKeyword("end") {
		return nil, fmt.Errorf("expected 'end' to close function")
	}
	p.pos += 3
	p.consumeTerminator()

	return &FuncDefNode{Name: name, Params: params, Body: body}, nil
}

func (p *Parser) parseBlockUntil(stopKeywords []string) ([]Node, error) {
	var nodes []Node
	for p.pos < len(p.input) {
		p.skipWhitespace()
		if p.pos >= len(p.input) {
			return nil, fmt.Errorf("unexpected EOF, expected block end")
		}

		matched := false
		for _, kw := range stopKeywords {
			if p.matchKeyword(kw) {
				matched = true
				break
			}
		}
		if matched {
			break
		}

		if p.matchKeyword("func") {
			p.pos += 4
			fnNode, err := p.parseFunctionDef()
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, fnNode)
			continue
		}
		if p.matchKeyword("if") {
			p.pos += 2 // <-- consume "if" critical bug number 99999
			ifNode, err := p.parseIfStatement()
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, ifNode)
			continue
		}
		if p.matchKeyword("while") {
			p.pos += 5
			whileNode, err := p.parseWhileLoop()
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, whileNode)
			continue
		}

		if p.matchKeyword("return") {
			p.pos += 6
			p.skipWhitespace()
			nextChar := ""
			if p.pos < len(p.input) {
				nextChar = string(p.input[p.pos])
			}
			if nextChar == ";" || nextChar == "\n" || nextChar == "" || isStopKeyword(p.input[p.pos:]) {
				nodes = append(nodes, &ReturnNode{Value: nil})
			} else {
				exprStr := p.readUntilTerminator()
				expr, err := parseExpression(exprStr)
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, &ReturnNode{Value: expr})
			}
			continue
		}
		if p.matchKeyword("break") {
			p.pos += 5
			nodes = append(nodes, &BreakNode{})
			p.consumeTerminator()
			continue
		}

		if p.matchKeyword("let") {
			p.pos += 3
			stmt, err := p.parseLetAssignment()
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, stmt)
			p.consumeTerminator()
			continue
		}

		stmt, err := p.parseAssignmentOrExpr()
		if err != nil {
			return nil, err
		}
		if stmt != nil {
			nodes = append(nodes, stmt)
		}
		p.consumeTerminator()
	}
	return nodes, nil
}

func parseExpression(s string) (Node, error) {
	tokens := tokenize(s)
	if len(tokens) == 0 {
		return nil, fmt.Errorf("empty expression")
	}
	parser := &ExprParser{tokens: tokens, pos: 0}
	return parser.parseOr()
}

type Token struct {
	Type  string
	Value string
}

func tokenize(s string) []Token {
	var tokens []Token
	for i := 0; i < len(s); {
		ch := s[i]
		if ch == ' ' || ch == '\t' {
			i++
			continue
		}

		if ch == '"' {
			start := i
			i++
			for i < len(s) && s[i] != '"' {
				i++
			}
			if i < len(s) {
				i++
			}
			tokens = append(tokens, Token{Type: "STRING", Value: s[start:i]})
			continue
		}

		if unicode.IsDigit(rune(ch)) || (ch == '.' && i+1 < len(s) && unicode.IsDigit(rune(s[i+1]))) {
			start := i
			for i < len(s) && (unicode.IsDigit(rune(s[i])) || s[i] == '.') {
				i++
			}
			tokens = append(tokens, Token{Type: "NUMBER", Value: s[start:i]})
			continue
		}

		if unicode.IsLetter(rune(ch)) || ch == '_' {
			start := i
			for i < len(s) && (unicode.IsLetter(rune(s[i])) || unicode.IsDigit(rune(s[i])) || s[i] == '_') {
				i++
			}
			val := s[start:i]
			if val == "and" || val == "or" || val == "not" {
				tokens = append(tokens, Token{Type: "KW", Value: val})
			} else {
				tokens = append(tokens, Token{Type: "WORD", Value: val})
			}
			continue
		}

		if i+1 < len(s) {
			two := s[i : i+2]
			if two == "==" || two == "!=" || two == "<=" || two == ">=" {
				tokens = append(tokens, Token{Type: "OP", Value: two})
				i += 2
				continue
			}
		}

		switch ch {
		case '+', '*', '/', '<', '>', '=':
			tokens = append(tokens, Token{Type: "OP", Value: string(ch)})
			i++
		case '-':
			tokens = append(tokens, Token{Type: "OP", Value: string(ch)})
			i++
		case '(':
			tokens = append(tokens, Token{Type: "LPAREN", Value: "("})
			i++
		case ')':
			tokens = append(tokens, Token{Type: "RPAREN", Value: ")"})
			i++
		case '[':
			tokens = append(tokens, Token{Type: "LBRACK", Value: "["})
			i++
		case ']':
			tokens = append(tokens, Token{Type: "RBRACK", Value: "]"})
			i++
		case ',':
			tokens = append(tokens, Token{Type: "COMMA", Value: ","})
			i++
		case '{':
			tokens = append(tokens, Token{Type: "LBRACE", Value: "{"})
			i++
		case '}':
			tokens = append(tokens, Token{Type: "RBRACE", Value: "}"})
			i++
		case ':':
			tokens = append(tokens, Token{Type: "COLON", Value: ":"})
			i++
		default:
			i++
		}
	}
	return tokens
}

type ExprParser struct {
	tokens []Token
	pos    int
}

func (p *ExprParser) peek() Token {
	if p.pos >= len(p.tokens) {
		return Token{Type: "EOF"}
	}
	return p.tokens[p.pos]
}

func (p *ExprParser) advance() Token {
	if p.pos < len(p.tokens) {
		t := p.tokens[p.pos]
		p.pos++
		return t
	}
	return Token{Type: "EOF"}
}

func (p *ExprParser) match(typ string, val ...string) bool {
	if p.pos >= len(p.tokens) {
		return false
	}
	t := p.tokens[p.pos]
	if t.Type != typ {
		return false
	}
	if len(val) > 0 && t.Value != val[0] {
		return false
	}
	return true
}

func (p *ExprParser) consume(typ string, val ...string) error {
	if !p.match(typ, val...) {
		t := p.peek()
		return fmt.Errorf("expected %s %v but got %s %s", typ, val, t.Type, t.Value)
	}
	p.pos++
	return nil
}

func (p *ExprParser) parseOr() (Node, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.match("KW", "or") {
		p.advance()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &BinaryOpNode{Left: left, Op: "or", Right: right}
	}
	return left, nil
}

func (p *ExprParser) parseAnd() (Node, error) {
	left, err := p.parseCompare()
	if err != nil {
		return nil, err
	}
	for p.match("KW", "and") {
		p.advance()
		right, err := p.parseCompare()
		if err != nil {
			return nil, err
		}
		left = &BinaryOpNode{Left: left, Op: "and", Right: right}
	}
	return left, nil
}

func (p *ExprParser) parseCompare() (Node, error) {
	left, err := p.parseAdd()
	if err != nil {
		return nil, err
	}
	if p.match("OP", "==") || p.match("OP", "!=") || p.match("OP", "<") || p.match("OP", ">") || p.match("OP", "<=") || p.match("OP", ">=") {
		tok := p.advance()
		right, err := p.parseAdd()
		if err != nil {
			return nil, err
		}
		return &BinaryOpNode{Left: left, Op: tok.Value, Right: right}, nil
	}
	return left, nil
}

func (p *ExprParser) parseAdd() (Node, error) {
	left, err := p.parseMul()
	if err != nil {
		return nil, err
	}
	for p.match("OP", "+") || p.match("OP", "-") {
		tok := p.advance()
		right, err := p.parseMul()
		if err != nil {
			return nil, err
		}
		left = &BinaryOpNode{Left: left, Op: tok.Value, Right: right}
	}
	return left, nil
}

func (p *ExprParser) parseMul() (Node, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for p.match("OP", "*") || p.match("OP", "/") {
		tok := p.advance()
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = &BinaryOpNode{Left: left, Op: tok.Value, Right: right}
	}
	return left, nil
}

func (p *ExprParser) parseUnary() (Node, error) {
	if p.match("KW", "not") {
		p.advance()
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &UnaryOpNode{Op: "not", Right: right}, nil
	}
	if p.match("OP", "-") {
		p.advance()
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &BinaryOpNode{Left: &LiteralNode{Value: 0.0, Type: "number"}, Op: "-", Right: right}, nil
	}
	return p.parseAccess()
}

func (p *ExprParser) parseAccess() (Node, error) {
	node, err := p.parseBase()
	if err != nil {
		return nil, err
	}

	for {
		if p.match("LPAREN") {
			p.advance()
			args := []Node{}
			if !p.match("RPAREN") {
				for {
					arg, err := p.parseOr()
					if err != nil {
						return nil, err
					}
					args = append(args, arg)
					if p.match("COMMA") {
						p.advance()
						continue
					}
					if p.match("RPAREN") {
						break
					}
					return nil, fmt.Errorf("expecting ')' or ',' in call")
				}
			}
			p.advance() // )
			if v, ok := node.(*VariableNode); ok {
				node = &CallNode{Target: v.Name, Args: args, CallType: "direct"}
			} else {
				node = &CallNode{Target: "", Args: args, CallType: "indirect", IndirectTarget: node}
			}
			continue
		}
		if p.match("LBRACK") {
			p.advance()
			index, err := p.parseOr()
			if err != nil {
				return nil, err
			}
			err = p.consume("RBRACK")
			if err != nil {
				return nil, err
			}
			node = &IndexAccessNode{Table: node, Index: index}
			continue
		}
		break
	}
	return node, nil
}

func (p *ExprParser) parseBase() (Node, error) {
	tok := p.peek()

	if tok.Type == "STRING" {
		p.advance()
		return &LiteralNode{Value: tok.Value[1 : len(tok.Value)-1], Type: "string"}, nil
	}
	if tok.Type == "NUMBER" {
		p.advance()
		val, _ := strconv.ParseFloat(tok.Value, 64)
		return &LiteralNode{Value: val, Type: "number"}, nil
	}
	if tok.Type == "WORD" || tok.Type == "KW" {
		p.advance()
		if tok.Value == "true" {
			return &LiteralNode{Value: 1.0, Type: "number"}, nil
		}
		if tok.Value == "false" {
			return &LiteralNode{Value: 0.0, Type: "number"}, nil
		}
		return &VariableNode{Name: tok.Value}, nil
	}

	if tok.Type == "LBRACE" {
		p.advance()
		keys := []string{}
		values := []Node{}
		for !p.match("RBRACE") {
			keyTok := p.peek()
			var keyStr string
			if keyTok.Type == "STRING" {
				keyStr = keyTok.Value[1 : len(keyTok.Value)-1]
				p.advance()
			} else if keyTok.Type == "WORD" {
				keyStr = keyTok.Value
				p.advance()
			} else {
				return nil, fmt.Errorf("expected string key in table literal")
			}
			if !p.match("COLON") {
				return nil, fmt.Errorf("expected ':' after key")
			}
			p.advance()
			val, err := p.parseOr()
			if err != nil {
				return nil, err
			}
			keys = append(keys, keyStr)
			values = append(values, val)
			if p.match("COMMA") {
				p.advance()
			}
		}
		p.advance() // }
		return &TableLiteralNode{Keys: keys, Values: values, IsArray: false}, nil
	}

	if tok.Type == "LBRACK" {
		p.advance()
		values := []Node{}
		if !p.match("RBRACK") {
			for {
				val, err := p.parseOr()
				if err != nil {
					return nil, err
				}
				values = append(values, val)
				if p.match("COMMA") {
					p.advance()
					continue
				}
				if p.match("RBRACK") {
					break
				}
				return nil, fmt.Errorf("expected ',' or ']' in array")
			}
		}
		p.advance() // ]
		return &TableLiteralNode{Values: values, IsArray: true}, nil
	}

	if tok.Type == "LPAREN" {
		p.advance()
		node, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if !p.match("RPAREN") {
			return nil, fmt.Errorf("expected ')' in expression")
		}
		p.advance()
		return node, nil
	}

	return nil, fmt.Errorf("unexpected token in expression: %v", tok)
}

func (p *Parser) skipWhitespace() {
	for p.pos < len(p.input) {
		c := p.input[p.pos]
		if c == ' ' || c == '\t' || c == '\r' {
			p.pos++
		} else if c == '\n' {
			p.line++
			p.pos++
		} else {
			break
		}
	}
}

func (p *Parser) matchKeyword(kw string) bool {
	if p.pos+len(kw) > len(p.input) {
		return false
	}
	sub := p.input[p.pos : p.pos+len(kw)]
	if sub != kw {
		return false
	}
	nextIdx := p.pos + len(kw)
	if nextIdx >= len(p.input) {
		return true
	}
	next := rune(p.input[nextIdx])
	return !unicode.IsLetter(next) && !unicode.IsDigit(next) && next != '_'
}

func (p *Parser) consumeTerminator() {
	p.skipWhitespace()
	if p.pos < len(p.input) {
		if p.input[p.pos] == ';' || p.input[p.pos] == '\n' {
			p.pos++
		}
	}
}

func (p *Parser) readUntilTerminator() string {
	start := p.pos
	inString := false
	escape := false

	for p.pos < len(p.input) {
		c := p.input[p.pos]

		if escape {
			escape = false
			p.pos++
			continue
		}

		if c == '\\' {
			escape = true
			p.pos++
			continue
		}

		if c == '"' {
			inString = !inString
			p.pos++
			continue
		}

		if !inString && (c == ';' || c == '\n' || c == '\r') {
			break
		}

		p.pos++
	}

	res := strings.TrimSpace(p.input[start:p.pos])
	return res
}

func (p *Parser) readUntilKeyword(kw string) string {
	start := p.pos
	depth := 0
	for p.pos < len(p.input) {
		if p.input[p.pos] == '(' {
			depth++
		} else if p.input[p.pos] == ')' {
			depth--
		}
		if depth == 0 && p.matchKeyword(kw) {
			break
		}
		p.pos++
	}
	res := strings.TrimSpace(p.input[start:p.pos])
	p.pos += len(kw) // should probably remove this ugly hack
	return res
}

func isVariable(s string) bool {
	if s == "" {
		return false
	}
	first := rune(s[0])
	if !unicode.IsLetter(first) && first != '_' {
		return false
	}
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return false
		}
	}
	kw := []string{"true", "false", "let", "print", "while", "do", "end", "if", "then", "else", "elseif", "func", "and", "or", "not", "return", "break"}
	for _, k := range kw {
		if s == k {
			return false
		}
	}
	return true
}

func isStopKeyword(s string) bool {
	for _, kw := range []string{"end", "else", "elseif", "while", "if", "func", "return", "break"} {
		if strings.HasPrefix(strings.TrimSpace(s), kw) {
			return true
		}
	}
	return false
}
