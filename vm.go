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

func toFloat64(val interface{}) float64 {
	if v, ok := val.(float64); ok {
		return v
	}
	if v, ok := val.(int); ok {
		return float64(v)
	}
	if v, ok := val.(int64); ok {
		return float64(v)
	}
	if v, ok := val.(int32); ok {
		return float64(v)
	}
	return 0.0
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

func (v *VM) Run(file string) error {
	if file != "" {
		if err := v.loadBytecode(file); err != nil {
			return err
		}
	}

	v.CallStack = []Frame{{Instructions: v.Instructions, Ip: 0, Sp: 0}}

	for len(v.CallStack) > 0 {
		f := &v.CallStack[len(v.CallStack)-1]

		for f.Ip < len(f.Instructions) {
			inst := f.Instructions[f.Ip]
			f.Ip++

			switch inst.Op {
			case OpConstant:
				idx := int(inst.Arg.(float64))
				if idx >= 0 && idx < len(v.Constants) {
					v.push(v.Constants[idx].Value)
				}
			case OpTable:
				v.push(make(map[string]interface{}, 4))
			case OpArray:
				count := int(inst.Arg.(float64))
				arr := make([]interface{}, count)
				base := v.Sp - count
				copy(arr, v.Stack[base:v.Sp])
				v.Sp = base
				v.push(arr)
			case OpCmpEq, OpCmpNe:
				b := v.pop()
				a := v.pop()
				eq := (a == b)
				if inst.Op == OpCmpNe {
					eq = !eq
				}
				if eq {
					v.push(1.0)
				} else {
					v.push(0.0)
				}
			case OpCmpLt:
				b := toFloat64(v.pop())
				a := toFloat64(v.pop())
				if a < b {
					v.push(1.0)
				} else {
					v.push(0.0)
				}
			case OpCmpLte:
				b := toFloat64(v.pop())
				a := toFloat64(v.pop())
				if a <= b {
					v.push(1.0)
				} else {
					v.push(0.0)
				}
			case OpCmpGt:
				b := toFloat64(v.pop())
				a := toFloat64(v.pop())
				if a > b {
					v.push(1.0)
				} else {
					v.push(0.0)
				}
			case OpCmpGte:
				b := toFloat64(v.pop())
				a := toFloat64(v.pop())
				if a >= b {
					v.push(1.0)
				} else {
					v.push(0.0)
				}
			case OpAdd:
				b := v.pop()
				a := v.pop()
				switch av := a.(type) {
				case float64:
					switch bv := b.(type) {
					case float64:
						v.push(av + bv)
					case int:
						v.push(av + float64(bv))
					case string:
						v.push(fmt.Sprintf("%v%v", av, bv))
					}
				case int:
					switch bv := b.(type) {
					case float64:
						v.push(float64(av) + bv)
					case int:
						v.push(float64(av + bv))
					case string:
						v.push(fmt.Sprintf("%v%v", a, b))
					case bool:
						v.push(fmt.Sprintf("%v%v", a, b))
					}
				case string:
					v.push(fmt.Sprintf("%v%v", a, b))
				}
			case OpSub:
				b := v.pop()
				a := v.pop()
				bf := toFloat64(b)
				af := toFloat64(a)
				v.push(af - bf)

			case OpMul:
				b := toFloat64(v.pop())
				a := toFloat64(v.pop())
				v.push(a * b)

			case OpDiv:
				b := toFloat64(v.pop())
				a := toFloat64(v.pop())
				if b == 0 {
					return fmt.Errorf("div by zero")
				}
				v.push(a / b)
			case OpNot:
				val := v.pop()
				isTrue := true
				if val == nil || val == 0.0 || val == false || val == "" {
					isTrue = false
				}
				if isTrue {
					v.push(0.0)
				} else {
					v.push(1.0)
				}
			case OpSetGlobal:
				val := v.pop()
				key := inst.Arg.(string)
				v.Globals[key] = val
			case OpGetGlobal:
				name := inst.Arg.(string)
				if val, ok := v.Globals[name]; ok {
					v.push(val)
				} else {
					v.push(nil)
				}
			case OpSetLocal:
				idx := int(inst.Arg.(float64))
				val := v.pop()
				v.Stack[f.Sp+idx] = val
			case OpGetLocal:
				idx := int(inst.Arg.(float64))
				val := v.Stack[f.Sp+idx]
				v.push(val)
			case OpGetIndex:
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
				}
			case OpSetIndex:
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
			case OpCall:
				count := int(toFloat64(v.pop()))
				target := inst.Arg.(string)

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
					continue
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
							continue
						}
					}
				}
				return fmt.Errorf("function '%s' not found", target)

			case OpCallIndirect:
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
						continue
					}
				}
				return fmt.Errorf("cannot call non-function")

			case OpReturn:
				var retVal interface{}
				if v.Sp > f.Sp {
					retVal = v.pop()
				} else {
					retVal = nil
				}
				v.CallStack = v.CallStack[:len(v.CallStack)-1]

				if len(v.CallStack) > 0 {
					callerFrame := &v.CallStack[len(v.CallStack)-1]
					v.Sp = callerFrame.Sp
					v.push(retVal)
				} else {
					v.Sp = 0
				}

				f.Ip = len(f.Instructions)
				continue
			case OpMakeFunc:
				idx := int(inst.Arg.(float64))
				entry := v.Constants[idx].Value
				fnObj := map[string]interface{}{
					"type":  "function",
					"entry": entry,
				}
				v.push(fnObj)
			case OpJump:
				target := int(toFloat64(inst.Arg))
				f.Ip = target
			case OpJumpIfFalse:
				cond := v.pop()
				isFalse := false
				if cond == nil || cond == 0.0 || cond == false {
					isFalse = true
				}
				if isFalse {
					target := int(toFloat64(inst.Arg))
					f.Ip = target
				}
			case OpPop:
				if v.Sp > 0 {
					v.Sp--
				}
			case OpHalt:
				return nil
			}
		}

		if len(v.CallStack) == 1 {
			return nil
		}
		v.CallStack = v.CallStack[:len(v.CallStack)-1]
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
