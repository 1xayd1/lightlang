package main

type OpCode byte

const (
	OpConstant OpCode = iota
	OpAdd
	OpSub
	OpMul
	OpDiv
	OpCmpEq
	OpCmpNe
	OpCmpLt
	OpCmpLte
	OpCmpGt
	OpCmpGte
	OpPop
	OpSetGlobal
	OpGetGlobal
	OpSetLocal
	OpGetLocal
	OpMakeFunc
	OpCall
	OpCallIndirect
	OpReturn
	OpPrint
	OpNop
	OpJump
	OpJumpIfFalse
	OpTable
	OpArray
	OpSetIndex
	OpGetIndex
	OpNot
	OpHalt
)

type Instruction struct {
	Op   OpCode
	Arg  interface{}
	Line int
}

type Constant struct {
	Value interface{}
	Type  string
}

type ForLoopNode struct {
	Init       Node
	Cond       Node
	Update     Node
	Body       []Node
	LoopVar    string
	Collection Node
	Type       string
}

type SymbolTable struct {
	Parent    *SymbolTable
	Locals    map[string]int // name -> stack index
	Globals   map[string]string
	IsFunc    bool
	NextLocal int
}

func NewSymbolTable(parent *SymbolTable, isFunc bool) *SymbolTable {
	return &SymbolTable{
		Parent:    parent,
		Locals:    make(map[string]int),
		Globals:   make(map[string]string),
		IsFunc:    isFunc,
		NextLocal: 0,
	}
}

func (s *SymbolTable) Define(name string, isLocal bool) int {
	if isLocal || s.IsFunc {
		idx := s.NextLocal
		s.Locals[name] = idx
		s.NextLocal++
		return idx
	} else {
		s.Globals[name] = "any"
		return -1
	}
}

func (s *SymbolTable) Resolve(name string) (isLocal bool, index int) {
	if idx, ok := s.Locals[name]; ok {
		return true, idx
	}
	if s.Parent != nil {
		return s.Parent.Resolve(name)
	}
	return false, -1 // global
}

type Node interface {
	TypeCheck(sym *SymbolTable) error
	Emit(b *Builder)
}

type LiteralNode struct {
	Value interface{}
	Type  string
}
type VariableNode struct{ Name string }
type UnaryOpNode struct {
	Op    string
	Right Node
}
type BinaryOpNode struct {
	Left  Node
	Op    string
	Right Node
}
type AssignmentNode struct {
	Name    string
	Expr    Node
	IsLocal bool
	Index   int
}
type IndexAssignNode struct {
	Table Node
	Index Node
	Value Node
}
type PrintNode struct{ Expr Node }
type ExprStmtNode struct{ Expr Node }
type CallNode struct {
	Target         string
	Args           []Node
	CallType       string
	IndirectTarget Node
}
type TableLiteralNode struct {
	Keys    []string
	Values  []Node
	IsArray bool
}
type IndexAccessNode struct {
	Table Node
	Index Node
}
type WhileLoopNode struct {
	Condition Node
	Body      []Node
}
type IfNode struct {
	Conditions []Node
	Bodies     [][]Node
	ElseBody   []Node
}
type FuncDefNode struct {
	Name   string
	Params []string
	Body   []Node
}
type ReturnNode struct{ Value Node }
type BreakNode struct{}

type Builder struct {
	Instructions []Instruction
	Constants    []Constant
	SymbolTable  *SymbolTable
	LoopStack    []int
}

func NewBuilder() *Builder {
	return &Builder{
		Instructions: []Instruction{},
		Constants:    []Constant{},
		SymbolTable:  NewSymbolTable(nil, false),
		LoopStack:    []int{},
	}
}

func (b *Builder) AddConstant(val interface{}, typ string) int {
	b.Constants = append(b.Constants, Constant{Value: val, Type: typ})
	return len(b.Constants) - 1
}

func (b *Builder) Emit(op OpCode, arg interface{}) {
	b.Instructions = append(b.Instructions, Instruction{Op: op, Arg: arg, Line: 0})
}

func (b *Builder) UpdateInstruction(idx int, arg interface{}) {
	if idx >= 0 && idx < len(b.Instructions) {
		b.Instructions[idx].Arg = arg
	}
}

func (b *Builder) Bytecode() ([]Instruction, []Constant) {
	return b.Instructions, b.Constants
}

func (n *LiteralNode) TypeCheck(sym *SymbolTable) error { return nil }
func (n *LiteralNode) Emit(b *Builder) {
	idx := b.AddConstant(n.Value, n.Type) // n.Value is interface{} like int, float64, string
	b.Emit(OpConstant, float64(idx))
}

func (n *VariableNode) TypeCheck(sym *SymbolTable) error { return nil }
func (n *VariableNode) Emit(b *Builder) {
	isLocal, idx := b.SymbolTable.Resolve(n.Name)
	if isLocal {
		b.Emit(OpGetLocal, float64(idx))
	} else {
		b.Emit(OpGetGlobal, n.Name)
	}
}

func (n *UnaryOpNode) TypeCheck(sym *SymbolTable) error { return n.Right.TypeCheck(sym) }
func (n *UnaryOpNode) Emit(b *Builder) {
	n.Right.Emit(b)
	if n.Op == "not" {
		b.Emit(OpNot, nil)
	}
}

func (n *BinaryOpNode) TypeCheck(sym *SymbolTable) error {
	n.Left.TypeCheck(sym)
	n.Right.TypeCheck(sym)
	return nil
}

func (n *BinaryOpNode) Emit(b *Builder) {
	n.Left.Emit(b)
	n.Right.Emit(b)
	switch n.Op {
	case "+":
		b.Emit(OpAdd, nil)
	case "-":
		b.Emit(OpSub, nil)
	case "*":
		b.Emit(OpMul, nil)
	case "/":
		b.Emit(OpDiv, nil)
	case "==":
		b.Emit(OpCmpEq, nil)
	case "!=":
		b.Emit(OpCmpNe, nil)
	case "<":
		b.Emit(OpCmpLt, nil)
	case "<=":
		b.Emit(OpCmpLte, nil)
	case ">":
		b.Emit(OpCmpGt, nil)
	case ">=":
		b.Emit(OpCmpGte, nil)
	case "and":
		b.Emit(OpMul, nil)
	case "or":
		b.Emit(OpAdd, nil)
	}
	return
}

func (n *ForLoopNode) TypeCheck(sym *SymbolTable) error {
	if n.Type == "in" {
		sym.Define(n.LoopVar, true)
		if n.Collection != nil {
			n.Collection.TypeCheck(sym)
		}
	} else {
		if n.Init != nil {
			n.Init.TypeCheck(sym)
		}
		if n.Cond != nil {
			n.Cond.TypeCheck(sym)
		}
		if n.Update != nil {
			n.Update.TypeCheck(sym)
		}
	}
	for _, stmt := range n.Body {
		stmt.TypeCheck(sym)
	}
	return nil
}

func (n *ForLoopNode) Emit(b *Builder) {
	switch n.Type {
	case "cstyle":
		n.emitCstyle(b)
	case "in":
		n.emitInLoop(b)
	default:
		n.emitCstyle(b)
	}
}

func (n *ForLoopNode) emitCstyle(b *Builder) {
	if n.Init != nil {
		if assign, ok := n.Init.(*AssignmentNode); ok {
			assign.Expr.Emit(b)
			isLocal, idx := b.SymbolTable.Resolve(assign.Name)
			if isLocal {
				b.Emit(OpSetLocal, float64(idx))
			} else {
				b.Emit(OpSetGlobal, assign.Name)
			}
			if isLocal {
				b.Emit(OpGetLocal, float64(idx))
			} else {
				b.Emit(OpGetGlobal, assign.Name)
			}
		} else {
			n.Init.Emit(b)
		}
		b.Emit(OpPop, nil)
	}

	startIdx := len(b.Instructions)
	b.LoopStack = append(b.LoopStack, startIdx)

	if n.Cond != nil {
		n.Cond.Emit(b)
		jumpFalseIdx := len(b.Instructions)
		b.Emit(OpJumpIfFalse, 0)

		for _, stmt := range n.Body {
			stmt.Emit(b)
		}

		if n.Update != nil {
			if assign, ok := n.Update.(*AssignmentNode); ok {
				assign.Expr.Emit(b)
				isLocal, idx := b.SymbolTable.Resolve(assign.Name)
				if isLocal {
					b.Emit(OpSetLocal, float64(idx))
				} else {
					b.Emit(OpSetGlobal, assign.Name)
				}
				b.Emit(OpPop, nil)
			} else {
				n.Update.Emit(b)
				b.Emit(OpPop, nil)
			}
		}

		b.Emit(OpJump, startIdx)

		exitIdx := len(b.Instructions)
		b.UpdateInstruction(jumpFalseIdx, exitIdx)
	} else {
		for _, stmt := range n.Body {
			stmt.Emit(b)
		}

		if n.Update != nil {
			if assign, ok := n.Update.(*AssignmentNode); ok {
				assign.Expr.Emit(b)
				isLocal, idx := b.SymbolTable.Resolve(assign.Name)
				if isLocal {
					b.Emit(OpSetLocal, float64(idx))
				} else {
					b.Emit(OpSetGlobal, assign.Name)
				}
				b.Emit(OpPop, nil)
			} else {
				n.Update.Emit(b)
				b.Emit(OpPop, nil)
			}
		}

		b.Emit(OpJump, startIdx)
	}

	b.LoopStack = b.LoopStack[:len(b.LoopStack)-1]
}

func (n *ForLoopNode) emitInLoop(b *Builder) {
	n.Collection.Emit(b)

	b.Emit(OpConstant, float64(b.AddConstant(1, "number")))
	b.Emit(OpCall, "len")

	counterIdx := b.SymbolTable.Define(n.LoopVar+"_counter", true)
	b.Emit(OpConstant, float64(b.AddConstant(0, "number")))
	b.Emit(OpSetLocal, float64(counterIdx))

	startIdx := len(b.Instructions)
	b.LoopStack = append(b.LoopStack, startIdx)

	b.Emit(OpGetLocal, float64(counterIdx))
	n.Collection.Emit(b)
	b.Emit(OpConstant, float64(b.AddConstant(1, "number")))
	b.Emit(OpCall, "len")
	b.Emit(OpCmpLt, nil)

	jumpFalseIdx := len(b.Instructions)
	b.Emit(OpJumpIfFalse, 0)

	n.Collection.Emit(b)
	b.Emit(OpGetLocal, float64(counterIdx))
	b.Emit(OpGetIndex, nil)

	loopVarIdx := b.SymbolTable.Define(n.LoopVar, true)
	b.Emit(OpSetLocal, float64(loopVarIdx))

	for _, stmt := range n.Body {
		stmt.Emit(b)
	}

	b.Emit(OpGetLocal, float64(counterIdx))
	b.Emit(OpConstant, float64(b.AddConstant(1, "number")))
	b.Emit(OpAdd, nil)
	b.Emit(OpSetLocal, float64(counterIdx))

	b.Emit(OpJump, startIdx)

	exitIdx := len(b.Instructions)
	b.UpdateInstruction(jumpFalseIdx, exitIdx)

	b.LoopStack = b.LoopStack[:len(b.LoopStack)-1]
}

func (n *AssignmentNode) TypeCheck(sym *SymbolTable) error {
	n.Expr.TypeCheck(sym)
	if n.IsLocal {
		sym.Define(n.Name, true)
	}
	return nil
}

func (n *AssignmentNode) Emit(b *Builder) {
	n.Expr.Emit(b)

	if n.IsLocal {
		index := b.SymbolTable.Define(n.Name, true)
		if index >= 0 {
			b.Emit(OpSetLocal, float64(index))
		} else {
			b.Emit(OpSetGlobal, n.Name)
		}
	} else {
		isLocal, index := b.SymbolTable.Resolve(n.Name)
		if isLocal {
			b.Emit(OpSetLocal, float64(index))
		} else {
			b.Emit(OpSetGlobal, n.Name)
		}
	}
}

func (n *IndexAssignNode) TypeCheck(sym *SymbolTable) error {
	n.Table.TypeCheck(sym)
	n.Index.TypeCheck(sym)
	n.Value.TypeCheck(sym)
	return nil
}

func (n *IndexAssignNode) Emit(b *Builder) {
	n.Table.Emit(b)
	n.Index.Emit(b)
	n.Value.Emit(b)
	b.Emit(OpSetIndex, nil)
}

func (n *IndexAccessNode) TypeCheck(sym *SymbolTable) error { return nil }
func (n *IndexAccessNode) Emit(b *Builder) {
	n.Table.Emit(b)
	n.Index.Emit(b)
	b.Emit(OpGetIndex, nil)
}

func (n *PrintNode) TypeCheck(sym *SymbolTable) error { return n.Expr.TypeCheck(sym) }
func (n *PrintNode) Emit(b *Builder) {
	n.Expr.Emit(b)
	b.Emit(OpPrint, nil)
}

func (n *ExprStmtNode) TypeCheck(sym *SymbolTable) error { return n.Expr.TypeCheck(sym) }
func (n *ExprStmtNode) Emit(b *Builder) {
	n.Expr.Emit(b)
	b.Emit(OpPop, nil)
}

func (n *CallNode) TypeCheck(sym *SymbolTable) error {
	for _, arg := range n.Args {
		arg.TypeCheck(sym)
	}
	return nil
}

func (n *CallNode) Emit(b *Builder) {
	for _, arg := range n.Args {
		arg.Emit(b)
	}
	b.Emit(OpConstant, float64(b.AddConstant(float64(len(n.Args)), "number")))

	if n.CallType == "direct" {
		b.Emit(OpCall, n.Target)
	} else {
		n.IndirectTarget.Emit(b)
		b.Emit(OpCallIndirect, nil)
	}
}

func (n *TableLiteralNode) TypeCheck(sym *SymbolTable) error { return nil }
func (n *TableLiteralNode) Emit(b *Builder) {
	if n.IsArray {
		for _, val := range n.Values {
			val.Emit(b)
		}
		b.Emit(OpArray, float64(len(n.Values)))
	} else {
		b.Emit(OpTable, nil)
		for i, k := range n.Keys {
			b.Emit(OpConstant, float64(b.AddConstant(k, "string")))
			n.Values[i].Emit(b)
			b.Emit(OpSetIndex, nil)
		}
	}
}

func (n *WhileLoopNode) TypeCheck(sym *SymbolTable) error {
	return n.Condition.TypeCheck(sym)
}

func (n *WhileLoopNode) Emit(b *Builder) {
	startIdx := len(b.Instructions)
	b.LoopStack = append(b.LoopStack, startIdx)

	n.Condition.Emit(b)
	jumpFalseIdx := len(b.Instructions)
	b.Emit(OpJumpIfFalse, 0)

	for _, stmt := range n.Body {
		stmt.Emit(b)
	}

	b.Emit(OpJump, startIdx)
	exitIdx := len(b.Instructions)
	b.UpdateInstruction(jumpFalseIdx, exitIdx)

	b.LoopStack = b.LoopStack[:len(b.LoopStack)-1]
}

func (n *IfNode) TypeCheck(sym *SymbolTable) error {
	for _, cond := range n.Conditions {
		cond.TypeCheck(sym)
	}
	for _, body := range n.Bodies {
		for _, stmt := range body {
			stmt.TypeCheck(sym)
		}
	}
	for _, stmt := range n.ElseBody {
		stmt.TypeCheck(sym)
	}
	return nil
}

func (n *IfNode) Emit(b *Builder) {
	var jumps []int
	var endJumps []int

	for i, cond := range n.Conditions {
		cond.Emit(b)
		jumpIdx := len(b.Instructions)
		b.Emit(OpJumpIfFalse, 0)
		jumps = append(jumps, jumpIdx)

		for _, stmt := range n.Bodies[i] {
			stmt.Emit(b)
		}

		if i < len(n.Conditions)-1 || len(n.ElseBody) > 0 {
			endJumpIdx := len(b.Instructions)
			b.Emit(OpJump, 0)
			endJumps = append(endJumps, endJumpIdx)
		}

		b.UpdateInstruction(jumpIdx, len(b.Instructions))
	}

	if len(n.ElseBody) > 0 {
		for _, stmt := range n.ElseBody {
			stmt.Emit(b)
		}
	}

	finalIdx := len(b.Instructions)
	for _, idx := range endJumps {
		b.UpdateInstruction(idx, finalIdx)
	}
}

func (n *FuncDefNode) TypeCheck(sym *SymbolTable) error {
	sym.Define(n.Name, false)
	return nil
}

func (n *FuncDefNode) Emit(b *Builder) {
	b.Emit(OpJump, 0)
	funcJumpIdx := len(b.Instructions) - 1

	prevSym := b.SymbolTable
	b.SymbolTable = NewSymbolTable(prevSym, true)

	for _, param := range n.Params {
		b.SymbolTable.Define(param, true)
	}

	startIp := len(b.Instructions)

	for _, stmt := range n.Body {
		stmt.Emit(b)
	}

	if len(b.Instructions) == 0 || b.Instructions[len(b.Instructions)-1].Op != OpReturn {
		b.Emit(OpReturn, nil)
	}

	funcBodyEnd := len(b.Instructions)
	b.SymbolTable = prevSym
	b.UpdateInstruction(funcJumpIdx, funcBodyEnd)

	idx := b.AddConstant(float64(startIp), "funcptr")
	b.Emit(OpMakeFunc, float64(idx))
	b.Emit(OpSetGlobal, n.Name)
}

func (n *ReturnNode) TypeCheck(sym *SymbolTable) error {
	if n.Value != nil {
		return n.Value.TypeCheck(sym)
	}
	return nil
}

func (n *ReturnNode) Emit(b *Builder) {
	if n.Value != nil {
		n.Value.Emit(b)
	} else {
		b.Emit(OpConstant, float64(b.AddConstant(nil, "nil")))
	}
	b.Emit(OpReturn, nil)
}

func (n *BreakNode) TypeCheck(sym *SymbolTable) error { return nil }
func (n *BreakNode) Emit(b *Builder) {
	b.Emit(OpJump, -1)
}
