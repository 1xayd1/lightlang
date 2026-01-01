package main

import (
	"fmt"
	"lightlang/builtins"
)

type Table map[string]interface{}

func NewTable() Table {
	return make(Table)
}

type Frame struct {
	Instructions []Instruction
	Ip           int
	Sp           int
	ArgCount     int
}

type VM struct {
	Instructions []Instruction
	Constants    []Constant
	Stack        []interface{}
	Sp           int
	CallStack    []Frame
	Globals      map[string]interface{}
}

func NewVM() *VM {
	return &VM{
		Stack:   make([]interface{}, 8192),
		Globals: make(map[string]interface{}, 128),
		Sp:      0,
	}
}

type opFunc func(v *VM, f *Frame) error

func toFloat64(val interface{}) float64 {
	switch v := val.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case int32:
		return float64(v)
	default:
		return 0.0
	}
}

func (v *VM) loadBytecode(file string) error {
	instructions, constants, err := LoadBytecode(file)
	if err != nil {
		return err
	}
	v.Instructions = instructions
	v.Constants = constants
	return nil
}

func (v *VM) precompile() []opFunc {
	ops := make([]opFunc, len(v.Instructions))
	for i, inst := range v.Instructions {
		ops[i] = v.makeOp(inst)
	}
	return ops
}

func adaptOp(
	genericHandler func(a, b interface{}) interface{},
	floatHandler func(a, b float64) float64,
) func(v *VM, f *Frame) error {

	var newfunc func(a, b interface{}) (interface{}, bool)

	newfunc = func(a, b interface{}) (interface{}, bool) { return nil, false }

	return func(v *VM, f *Frame) error {
		b := v.pop()
		a := v.pop()

		if res, ok := newfunc(a, b); ok {
			v.push(res)
			return nil
		}
		switch at := a.(type) {
		case float64:
			if bt, ok := b.(float64); ok {
				if floatHandler != nil {
					newfunc = func(x, y interface{}) (interface{}, bool) {
						f1, ok1 := x.(float64)
						f2, ok2 := y.(float64)
						if ok1 && ok2 {
							return floatHandler(f1, f2), true
						}
						return nil, false
					}
					v.push(floatHandler(at, bt))
					return nil
				}
			}
		}

		v.push(genericHandler(a, b))
		return nil
	}
}

func (v *VM) makeOp(inst Instruction) opFunc {
	switch inst.Op {
	case OpConstant:
		idx := int(inst.Arg.(float64))
		val := v.Constants[idx].Value
		return func(v *VM, f *Frame) error {
			v.push(val)
			return nil
		}

	case OpTable:
		return func(v *VM, f *Frame) error {
			v.push(make(map[string]interface{}, 4))
			return nil
		}

	case OpArray:
		count := int(inst.Arg.(float64))
		return func(v *VM, f *Frame) error {
			arr := make([]interface{}, count)
			base := v.Sp - count
			copy(arr, v.Stack[base:v.Sp])
			v.Sp = base
			v.push(arr)
			return nil
		}

	case OpCmpEq:
		return func(v *VM, f *Frame) error {
			b := v.pop()
			a := v.pop()
			if a == b {
				v.push(1.0)
			} else {
				v.push(0.0)
			}
			return nil
		}

	case OpCmpNe:
		return func(v *VM, f *Frame) error {
			b := v.pop()
			a := v.pop()
			if a != b {
				v.push(1.0)
			} else {
				v.push(0.0)
			}
			return nil
		}

	case OpCmpLt:
		return func(v *VM, f *Frame) error {
			b := v.pop()
			a := v.pop()
			if af, ok1 := a.(float64); ok1 {
				if bf, ok2 := b.(float64); ok2 {
					if af < bf {
						v.push(1.0)
					} else {
						v.push(0.0)
					}
					return nil
				}
			}
			if toFloat64(a) < toFloat64(b) {
				v.push(1.0)
			} else {
				v.push(0.0)
			}
			return nil
		}
	case OpCmpLte:
		return func(v *VM, f *Frame) error {
			b := toFloat64(v.pop())
			a := toFloat64(v.pop())
			if a <= b {
				v.push(1.0)
			} else {
				v.push(0.0)
			}
			return nil
		}

	case OpCmpGt:
		return func(v *VM, f *Frame) error {
			b := toFloat64(v.pop())
			a := toFloat64(v.pop())
			if a > b {
				v.push(1.0)
			} else {
				v.push(0.0)
			}
			return nil
		}

	case OpCmpGte:
		return func(v *VM, f *Frame) error {
			b := toFloat64(v.pop())
			a := toFloat64(v.pop())
			if a >= b {
				v.push(1.0)
			} else {
				v.push(0.0)
			}
			return nil
		}

	case OpAdd:
		genericAdd := func(a, b interface{}) interface{} {
			switch av := a.(type) {
			case float64:
				switch bv := b.(type) {
				case float64:
					return av + bv
				case int:
					return av + float64(bv)
				case string:
					return fmt.Sprintf("%v%v", av, bv)
				}
			case int:
				switch bv := b.(type) {
				case float64:
					return float64(av) + bv
				case int:
					return float64(av + bv)
				case string:
					return fmt.Sprintf("%v%v", a, b)
				case bool:
					return fmt.Sprintf("%v%v", a, b)
				}
			case string:
				if bs, ok := b.(string); ok {
					return av + bs
				}
				return fmt.Sprintf("%s%v", av, b)
			}
			return fmt.Sprintf("%v%v", a, b)
		}

		return adaptOp(genericAdd, func(a, b float64) float64 {
			return a + b
		})

	case OpSub:
		genericSub := func(a, b interface{}) interface{} {
			return toFloat64(a) - toFloat64(b)
		}
		return adaptOp(genericSub, func(a, b float64) float64 {
			return a - b
		})

	case OpMul:
		genericMul := func(a, b interface{}) interface{} {
			return toFloat64(a) * toFloat64(b)
		}
		return adaptOp(genericMul, func(a, b float64) float64 {
			return a * b
		})

	case OpDiv:
		return func(v *VM, f *Frame) error {
			b := v.pop()
			a := v.pop()

			if af, ok1 := a.(float64); ok1 {
				if bf, ok2 := b.(float64); ok2 {
					if bf == 0 {
						return fmt.Errorf("div by zero")
					}
					v.push(af / bf)
					return nil
				}
			}
			bf := toFloat64(b)
			if bf == 0 {
				return fmt.Errorf("div by zero")
			}
			af := toFloat64(a)
			v.push(af / bf)
			return nil
		}

	case OpNot:
		return func(v *VM, f *Frame) error {
			val := v.pop()
			if val == nil || val == 0.0 || val == false || val == "" {
				v.push(1.0)
			} else {
				v.push(0.0)
			}
			return nil
		}

	case OpSetGlobal:
		key := inst.Arg.(string)
		return func(v *VM, f *Frame) error {
			v.Globals[key] = v.pop()
			return nil
		}

	case OpGetGlobal:
		name := inst.Arg.(string)
		return func(v *VM, f *Frame) error {
			if val, ok := v.Globals[name]; ok {
				v.push(val)
			} else {
				v.push(nil)
			}
			return nil
		}

	case OpSetLocal:
		idx := int(inst.Arg.(float64))
		return func(v *VM, f *Frame) error {
			v.Stack[f.Sp+idx] = v.pop()
			return nil
		}

	case OpGetLocal:
		idx := int(inst.Arg.(float64))
		return func(v *VM, f *Frame) error {
			v.push(v.Stack[f.Sp+idx])
			return nil
		}

	case OpGetIndex:
		return func(v *VM, f *Frame) error {
			index := v.pop()
			table := v.pop()
			switch t := table.(type) {
			case []interface{}:
				i := int(toFloat64(index))
				if i >= 0 && i < len(t) {
					v.push(t[i])
				} else {
					v.push(nil)
				}
			case map[string]interface{}:
				key := fmt.Sprintf("%v", index)
				if val, ok := t[key]; ok {
					v.push(val)
				} else {
					v.push(nil)
				}
			default:
				v.push(nil)
			}
			return nil
		}

	case OpSetIndex:
		return func(v *VM, f *Frame) error {
			val := v.pop()
			index := v.pop()
			table := v.pop()
			switch t := table.(type) {
			case []interface{}:
				var i int
				switch idx := index.(type) {
				case float64:
					i = int(idx)
				case int:
					i = idx
				default:
					i = int(toFloat64(index))
				}
				if i >= 0 && i < len(t) {
					t[i] = val
				}
				v.push(t)
			case map[string]interface{}:
				key := fmt.Sprintf("%v", index)
				t[key] = val
				v.push(t)
			}
			return nil
		}

	case OpCall:
		target := inst.Arg.(string)
		return func(v *VM, f *Frame) error {
			count := int(toFloat64(v.pop()))
			if fn, ok := builtins.Builtins[target]; ok {
				args := make([]interface{}, count)
				base := v.Sp - count
				copy(args, v.Stack[base:v.Sp])
				v.Sp = base
				res, err := fn(args)
				if err != nil {
					return err
				}
				if res != nil {
					v.push(res)
				} else {
					v.push(nil)
				}
				return nil
			}
			if val, ok := v.Globals[target]; ok {
				if fnMeta, ok := val.(map[string]interface{}); ok {
					if t, ok := fnMeta["type"]; ok && t == "function" {
						entry := int(fnMeta["entry"].(float64))
						baseSp := v.Sp - count
						v.CallStack = append(v.CallStack, Frame{
							Instructions: v.Instructions,
							Ip:           entry,
							Sp:           baseSp,
							ArgCount:     count,
						})
						return nil
					}
				}
			}
			return fmt.Errorf("function '%s' not found", target)
		}

	case OpCallIndirect:
		return func(v *VM, f *Frame) error {
			count := int(v.pop().(float64))
			val := v.pop()
			if fnMeta, ok := val.(map[string]interface{}); ok {
				if t, ok := fnMeta["type"]; ok && t == "function" {
					entry := int(fnMeta["entry"].(float64))
					baseSp := v.Sp - count
					v.CallStack = append(v.CallStack, Frame{
						Instructions: v.Instructions,
						Ip:           entry,
						Sp:           baseSp,
						ArgCount:     count,
					})
					return nil
				}
			}
			return fmt.Errorf("cannot call non-function")
		}

	case OpReturn:
		return func(v *VM, f *Frame) error {
			frameSp := f.Sp
			var retVal interface{}
			if v.Sp > frameSp {
				retVal = v.pop()
			} else {
				retVal = nil
			}
			v.CallStack = v.CallStack[:len(v.CallStack)-1]
			if len(v.CallStack) > 0 {
				v.Sp = frameSp
				v.push(retVal)
			} else {
				v.Sp = 0
			}
			return nil
		}

	case OpMakeFunc:
		idx := int(inst.Arg.(float64))
		entry := v.Constants[idx].Value
		return func(v *VM, f *Frame) error {
			fnObj := map[string]interface{}{
				"type":  "function",
				"entry": entry,
			}
			v.push(fnObj)
			return nil
		}

	case OpJump:
		target := int(toFloat64(inst.Arg))
		return func(v *VM, f *Frame) error {
			f.Ip = target
			return nil
		}

	case OpJumpIfFalse:
		target := int(toFloat64(inst.Arg))
		return func(v *VM, f *Frame) error {
			cond := v.pop()
			if cond == nil || cond == 0.0 || cond == false || cond == "" {
				f.Ip = target
			}
			return nil
		}

	case OpPop:
		return func(v *VM, f *Frame) error {
			if v.Sp > 0 {
				v.Sp--
			}
			return nil
		}

	case OpHalt:
		return func(v *VM, f *Frame) error { return fmt.Errorf("_HALT_") }
	}

	return func(v *VM, f *Frame) error { return nil }
}

func (v *VM) Run(file string) error {
	if file != "" {
		if err := v.loadBytecode(file); err != nil {
			return err
		}
	}
	compiledOps := v.precompile()
	v.CallStack = []Frame{{Instructions: v.Instructions, Ip: 0, Sp: 0}}
	for len(v.CallStack) > 0 {
		f := &v.CallStack[len(v.CallStack)-1]
		currentStackDepth := len(v.CallStack)
		for f.Ip < len(compiledOps) {
			op := compiledOps[f.Ip]
			f.Ip++
			if err := op(v, f); err != nil {
				if err.Error() == "_HALT_" {
					return nil
				}
				return err
			}
			if len(v.CallStack) != currentStackDepth {
				break
			}
		}
	}
	return nil
}

func (v *VM) push(val interface{}) {
	if v.Sp >= len(v.Stack) {
		newStack := make([]interface{}, len(v.Stack)+(len(v.Stack)>>1))
		copy(newStack, v.Stack)
		v.Stack = newStack
	}
	v.Stack[v.Sp] = val
	v.Sp++
}

func (v *VM) pop() interface{} {
	if v.Sp <= 0 {
		panic("Stack Underflow")
	}
	v.Sp--
	return v.Stack[v.Sp]
}
