package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// Magic: 4 bytes "LLBC"
// Version: 1 byte (major in high 4 bits, minor in low 4 bits)
// Constant pool count: variable-length uint (1-4 bytes)
// Instruction count: variable-length uint (1-4 bytes)
// Constant pool:
//   For each constant:
//     Type+flags: 1 byte (type in low 3 bits, flags in high bits)
//     Data: variable (optimized based on type)
// Instructions:
//   For each instruction:
//     Opcode: 1 byte (high bit indicates has arg)
//     Line: variable-length uint (1-2 bytes, 0-65535)
//     If has arg:
//       Arg type: 2 bits (0=const index, 1=int, 2=float, 3=string)
//       Arg data: variable

const (
	MagicHeader           = "LLBC"
	VersionMajor    uint8 = 2
	VersionMinor    uint8 = 3
	VersionCombined       = (VersionMajor << 4) | (VersionMinor & 0x0F)

	ConstTypeNumber  = 0
	ConstTypeString  = 1
	ConstTypeFuncPtr = 2
	ConstTypeBool    = 3
	ConstTypeNil     = 4

	ConstFlagSmallInt = 0x80
	ConstFlagShortStr = 0x40

	ArgTypeConst  = 0
	ArgTypeInt    = 1
	ArgTypeFloat  = 2
	ArgTypeString = 3
)

type BytecodeWriter struct {
	writer io.Writer
}

func NewBytecodeWriter(w io.Writer) *BytecodeWriter {
	return &BytecodeWriter{writer: w}
}

func (bw *BytecodeWriter) WriteBytecode(instructions []Instruction, constants []Constant) error {
	if _, err := bw.writer.Write([]byte(MagicHeader)); err != nil {
		return err
	}

	if err := bw.WriteUint8(VersionCombined); err != nil {
		return err
	}

	if err := bw.writeVarUint(uint32(len(constants))); err != nil {
		return err
	}
	if err := bw.writeVarUint(uint32(len(instructions))); err != nil {
		return err
	}

	for _, c := range constants {
		switch c.Type {
		case "number":
			if val, ok := c.Value.(int); ok && val >= -127 && val <= 127 {
				if err := bw.WriteUint8(ConstTypeNumber | ConstFlagSmallInt); err != nil {
					return err
				}
				if err := bw.WriteUint8(uint8(int8(val))); err != nil {
					return err
				}
			} else {
				var fval float64
				if val, ok := c.Value.(float64); ok {
					fval = val
				} else if val, ok := c.Value.(int); ok {
					fval = float64(val)
				}
				if err := bw.WriteUint8(ConstTypeNumber); err != nil {
					return err
				}
				if err := binary.Write(bw.writer, binary.LittleEndian, fval); err != nil {
					return err
				}
			}

		case "string":
			str := c.Value.(string)
			if len(str) <= 255 {
				if err := bw.WriteUint8(ConstTypeString | ConstFlagShortStr); err != nil {
					return err
				}
				if err := bw.WriteUint8(uint8(len(str))); err != nil {
					return err
				}
			} else {
				if err := bw.WriteUint8(ConstTypeString); err != nil {
					return err
				}
				if err := bw.writeVarUint(uint32(len(str))); err != nil {
					return err
				}
			}
			if _, err := bw.writer.Write([]byte(str)); err != nil {
				return err
			}

		case "funcptr":
			if err := bw.WriteUint8(ConstTypeFuncPtr); err != nil {
				return err
			}
			var val uint32
			if v, ok := c.Value.(float64); ok {
				val = uint32(v)
			} else if v, ok := c.Value.(int); ok {
				val = uint32(v)
			}
			if err := bw.writeVarUint(val); err != nil {
				return err
			}

		case "bool":
			if err := bw.WriteUint8(ConstTypeBool); err != nil {
				return err
			}
			var val byte = 0
			if c.Value == true {
				val = 1
			}
			if err := bw.WriteUint8(val); err != nil {
				return err
			}

		case "nil":
			if err := bw.WriteUint8(ConstTypeNil); err != nil {
				return err
			}
		}
	}

	for _, inst := range instructions {
		opcode := byte(inst.Op)
		hasArg := inst.Arg != nil
		if hasArg {
			opcode |= 0x80
		}
		if err := bw.WriteUint8(opcode); err != nil {
			return err
		}

		if err := bw.writeVarUint16(uint16(inst.Line)); err != nil {
			return err
		}

		if hasArg {
			switch arg := inst.Arg.(type) {
			case float64:
				if arg == float64(int32(arg)) {
					if err := bw.WriteUint8(ArgTypeInt); err != nil {
						return err
					}
					if err := binary.Write(bw.writer, binary.LittleEndian, int32(arg)); err != nil {
						return err
					}
				} else {
					if err := bw.WriteUint8(ArgTypeFloat); err != nil {
						return err
					}
					if err := binary.Write(bw.writer, binary.LittleEndian, arg); err != nil {
						return err
					}
				}

			case int:
				if err := bw.WriteUint8(ArgTypeInt); err != nil {
					return err
				}
				if err := bw.writeVarInt(int32(arg)); err != nil {
					return err
				}

			case string:
				if err := bw.WriteUint8(ArgTypeString); err != nil {
					return err
				}
				if err := bw.writeVarUint(uint32(len(arg))); err != nil {
					return err
				}
				if _, err := bw.writer.Write([]byte(arg)); err != nil {
					return err
				}

			default:
				if f, ok := arg.(float64); ok {
					if err := bw.WriteUint8(ArgTypeConst); err != nil {
						return err
					}
					if err := bw.writeVarUint(uint32(f)); err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

func (bw *BytecodeWriter) WriteUint8(val uint8) error {
	_, err := bw.writer.Write([]byte{val})
	return err
}

func (bw *BytecodeWriter) writeVarUint(val uint32) error {
	for val >= 0x80 {
		if err := bw.WriteUint8(uint8(val) | 0x80); err != nil {
			return err
		}
		val >>= 7
	}
	return bw.WriteUint8(uint8(val))
}

func (bw *BytecodeWriter) writeVarUint16(val uint16) error {
	if val < 0x80 {
		return bw.WriteUint8(uint8(val))
	}
	if err := bw.WriteUint8(uint8(val) | 0x80); err != nil {
		return err
	}
	return bw.WriteUint8(uint8(val >> 7))
}

func (bw *BytecodeWriter) writeVarInt(val int32) error {
	uval := uint32(val) << 1
	if val < 0 {
		uval = ^uval
	}
	return bw.writeVarUint(uval)
}

type BytecodeReader struct {
	reader io.Reader
}

func NewBytecodeReader(r io.Reader) *BytecodeReader {
	return &BytecodeReader{reader: r}
}

func (br *BytecodeReader) ReadBytecode() ([]Instruction, []Constant, error) {
	magic := make([]byte, 4)
	if _, err := io.ReadFull(br.reader, magic); err != nil {
		return nil, nil, err
	}
	if string(magic) != MagicHeader {
		return nil, nil, fmt.Errorf("invalid bytecode file: bad magic")
	}

	version, err := br.ReadUint8()
	if err != nil {
		return nil, nil, err
	}
	major := version >> 4
	minor := version & 0x0F
	if major != VersionMajor {
		return nil, nil, fmt.Errorf("incompatible bytecode version: %d.%d", major, minor)
	}

	constantCount, err := br.readVarUint()
	if err != nil {
		return nil, nil, err
	}
	instructionCount, err := br.readVarUint()
	if err != nil {
		return nil, nil, err
	}

	constants := make([]Constant, constantCount)
	for i := range constants {
		constInfo, err := br.ReadUint8()
		if err != nil {
			return nil, nil, err
		}

		constType := constInfo & 0x07
		flags := constInfo & 0xF8

		switch constType {
		case ConstTypeNumber:
			if flags&ConstFlagSmallInt != 0 {
				val, err := br.ReadUint8()
				if err != nil {
					return nil, nil, err
				}
				constants[i] = Constant{Value: int(int8(val)), Type: "number"}
			} else {
				var val float64
				if err := binary.Read(br.reader, binary.LittleEndian, &val); err != nil {
					return nil, nil, err
				}
				constants[i] = Constant{Value: val, Type: "number"}
			}

		case ConstTypeString:
			var strLen uint32
			if flags&ConstFlagShortStr != 0 {
				val, err := br.ReadUint8()
				if err != nil {
					return nil, nil, err
				}
				strLen = uint32(val)
			} else {
				strLen, err = br.readVarUint()
				if err != nil {
					return nil, nil, err
				}
			}
			strBytes := make([]byte, strLen)
			if _, err := io.ReadFull(br.reader, strBytes); err != nil {
				return nil, nil, err
			}
			constants[i] = Constant{Value: string(strBytes), Type: "string"}

		case ConstTypeFuncPtr:
			val, err := br.readVarUint()
			if err != nil {
				return nil, nil, err
			}
			constants[i] = Constant{Value: float64(val), Type: "funcptr"}

		case ConstTypeBool:
			val, err := br.ReadUint8()
			if err != nil {
				return nil, nil, err
			}
			constants[i] = Constant{Value: val == 1, Type: "bool"}

		case ConstTypeNil:
			constants[i] = Constant{Value: nil, Type: "nil"}
		}
	}

	instructions := make([]Instruction, instructionCount)
	for i := range instructions {
		opcode, err := br.ReadUint8()
		if err != nil {
			return nil, nil, err
		}

		hasArg := (opcode & 0x80) != 0
		opcode &^= 0x80

		line, err := br.readVarUint16()
		if err != nil {
			return nil, nil, err
		}

		var arg interface{}
		if hasArg {
			argType, err := br.ReadUint8()
			if err != nil {
				return nil, nil, err
			}

			switch argType & 0x03 {
			case ArgTypeConst:
				idx, err := br.readVarUint()
				if err != nil {
					return nil, nil, err
				}
				arg = float64(idx)

			case ArgTypeInt:
				// Read variable-length int
				uval, err := br.readVarUint()
				if err != nil {
					return nil, nil, err
				}
				val := int32(uval >> 1)
				if (uval & 1) != 0 {
					val = ^val
				}
				arg = float64(val)

			case ArgTypeFloat:
				var val float64
				if err := binary.Read(br.reader, binary.LittleEndian, &val); err != nil {
					return nil, nil, err
				}
				arg = val

			case ArgTypeString:
				strLen, err := br.readVarUint()
				if err != nil {
					return nil, nil, err
				}
				strBytes := make([]byte, strLen)
				if _, err := io.ReadFull(br.reader, strBytes); err != nil {
					return nil, nil, err
				}
				arg = string(strBytes)
			}
		}

		instructions[i] = Instruction{
			Op:   OpCode(opcode),
			Arg:  arg,
			Line: int(line),
		}
	}

	return instructions, constants, nil
}

func (br *BytecodeReader) ReadUint8() (uint8, error) {
	var buf [1]byte
	_, err := io.ReadFull(br.reader, buf[:])
	return buf[0], err
}

func (br *BytecodeReader) readVarUint() (uint32, error) {
	var result uint32
	var shift uint
	for {
		b, err := br.ReadUint8()
		if err != nil {
			return 0, err
		}
		result |= uint32(b&0x7F) << shift
		if b&0x80 == 0 {
			break
		}
		shift += 7
	}
	return result, nil
}

func (br *BytecodeReader) readVarUint16() (uint16, error) {
	first, err := br.ReadUint8()
	if err != nil {
		return 0, err
	}
	if first < 0x80 {
		return uint16(first), nil
	}
	second, err := br.ReadUint8()
	if err != nil {
		return 0, err
	}
	return uint16(first&0x7F) | (uint16(second) << 7), nil
}

func SaveBytecode(filename string, instructions []Instruction, constants []Constant) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := NewBytecodeWriter(file)
	return writer.WriteBytecode(instructions, constants)
}

func LoadBytecode(filename string) ([]Instruction, []Constant, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	reader := NewBytecodeReader(file)
	return reader.ReadBytecode()
}

type bytesBuffer struct {
	data []byte
	pos  int
}

func (b *bytesBuffer) Write(p []byte) (n int, err error) {
	if b.pos+len(p) > len(b.data) {
		newSize := len(b.data) * 2
		if newSize < b.pos+len(p) {
			newSize = b.pos + len(p)
		}
		newData := make([]byte, newSize)
		copy(newData, b.data)
		b.data = newData
	}
	n = copy(b.data[b.pos:], p)
	b.pos += n
	return n, nil
}
