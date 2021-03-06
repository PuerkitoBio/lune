package vm

import (
	"fmt"

	"github.com/mna/lune/types"
)

var (
	_BINOPS = [...]byte{
		types.OP_ADD: '+',
		types.OP_SUB: '-',
		types.OP_MUL: '*',
		types.OP_DIV: '/',
		types.OP_MOD: '%',
		types.OP_POW: '^',
	}

	_CMPOPS = [...]func(types.Value, types.Value) bool{
		types.OP_EQ: areEqual,
		types.OP_LT: isLessThan,
		types.OP_LE: isLessEqual,
	}
)

func doJump(s *types.State, args types.Args, e int) {
	if asBool(args.Ax) {
		closeUpvalues(s, s.CI.Base+args.Ax-1)
	}
	s.CI.PC += args.Bx + e
}

func preCall(s *types.State, args types.Args, nRets int) bool {
	switch f := (*args.A).(type) {
	case types.GoFunc:
		// Go function call
		callGoFunc(s, f, s.CI.Base+args.Ax+1, nRets)
		return true
	case *types.Closure:
		// Lune function call
		s.NewCallInfo(f, s.CI.Base+args.Ax, nRets)
		// TODO : Metamethods
	}
	return false
}

func posCall(s *types.State, firstResult int) int {
	// TODO : See luaD_poscall in ldo.c, the hook debugging is not implemented for now
	res := s.CI.FuncIndex
	wanted := s.CI.NumResults
	s.CI = s.CI.Prev
	// Set results in the right slots on the stack
	var i int
	for i = wanted; i != 0 && firstResult < s.Top; i-- {
		s.Stack[res] = s.Stack[firstResult]
		res, firstResult = res+1, firstResult+1
	}
	// Complete missing results with nils
	for ; i > 0; i-- {
		s.Stack[res] = nil
		res++
	}
	s.Top = res
	return wanted - types.LUNE_MULTRET
}

func closeUpvalues(s *types.State, funcIdx int) {
	for _, stkIdx := range s.OpenUpVals {
		// TODO : Could be optimized if openupvals are in order (greatest first)
		if stkIdx >= funcIdx {
			// Garbage collect stuff omitted...
			// Should use a linked list instead...
			// TODO : Still not clear how upvalues work once closed, where do they live?
			// will wait for better understanding of closure calls and upvalue lookup...
		}
	}
}

func findUpval() types.Value {
	// TODO : Implement...
	return nil
}

func pushClosure(s *types.State, p *types.Prototype, ra *types.Value) {
	parentCl := s.CI.Cl
	cl := types.NewClosure(p)
	// Push the new closure onto the stack, in its slot
	*ra = cl

	// Assign upvalues
	for i, uv := range p.Upvalues {
		if asBool(int(uv.Instack)) {
			// Upval is a local variable
			cl.UpVals[i] = findUpval()
		} else {
			// Get upval from enclosing function's upvalues
			cl.UpVals[i] = parentCl.UpVals[uv.Idx]
		}
	}
}

func callGoFunc(s *types.State, f types.GoFunc, base, nRets int) {
	var in []types.Value
	for i := base; i < s.Top; i++ {
		in = append(in, s.Stack[i])
	}
	out := f(in)
	// Out values replace the stack values starting at the Go Func index (base - 1)
	// nRets values are expected, stop at this count, and fill with nils if necessary
	s.Top = base - 1
	for i := 0; i < nRets; i++ {
		if i < len(out) {
			s.Stack[s.Top] = out[i]
			s.Top++
		} else {
			s.Stack[s.Top] = nil
			s.Top++
		}
	}
}

func Execute(s *types.State) {
	// Start with entry point (position 0)
	s.NewCallInfo(s.Stack[0].(*types.Closure), 0, 0)

newFrame:
	var i types.Instruction
	var op types.OpCode
	var args types.Args

	for {
		i = s.CI.Cl.P.Code[s.CI.PC]
		op = i.GetOpCode()
		s.CI.PC++
		s.Dump()
		s.OpCodeDebug = append(s.OpCodeDebug, op)
		args = i.GetArgs(s)

		fmt.Printf("ax: %d, bx: %d, cx: %d\n", args.Ax, args.Bx, args.Cx)

		switch op {
		case types.OP_MOVE, types.OP_LOADK, types.OP_GETUPVAL:
			// A B | R(A) := R(B)
			// A Bx | R(A) := Kst(Bx)
			// A B | R(A) := UpValue[B]
			// Status: done
			*args.A = *args.B
			if op == types.OP_MOVE {
				fmt.Printf("%-10sR(A)=%v R(B)=%v\n", op, *args.A, *args.B)
			} else if op == types.OP_LOADK {
				fmt.Printf("%-10sR(A)=%v Kst(Bx)=%v\n", op, *args.A, *args.B)
			} else {
				fmt.Printf("%-10sR(A)=%v U(B)=%v\n", op, *args.A, *args.B)
			}

		case types.OP_LOADKx:
			// A | R(A) := Kst(extra arg)
			// Special instruction: must always be followed by OP_EXTRAARG
			// Status: untested
			if i2 := s.CI.Cl.P.Code[s.CI.PC]; i2.GetOpCode() != types.OP_EXTRAARG {
				panic(fmt.Sprintf("%s: expected OP_EXTRAARG as next instruction, found %s", op, i2.GetOpCode()))
			} else {
				s.CI.PC++
				s.OpCodeDebug = append(s.OpCodeDebug, i2.GetOpCode())
				ax := i2.GetArgAx()
				*args.A = s.CI.Cl.P.Ks[ax]
				fmt.Printf("%-10sR(A)=%v EXTRAARG=%v\n", op, *args.A, ax)
			}

		case types.OP_LOADBOOL:
			// A B C | R(A) := (Bool)B; if (C) PC++
			// Status: done
			*args.A = asBool(args.Bx)

			// Skip next instruction if C is true
			if asBool(args.Cx) {
				s.CI.PC++
			}
			fmt.Printf("%-10sR(A)=%v B=%v C=%v\n", op, *args.A, args.Bx, args.Cx)

		case types.OP_LOADNIL:
			// A B | R(A) := ... := R(B) := nil
			// Status: done
			for j := 0; j <= args.Bx; j++ {
				s.CI.Frame[args.Ax+j] = nil
			}
			fmt.Printf("%-10sA=%v B=%v\n", op, args.Ax, args.Bx)

		case types.OP_GETTABUP, types.OP_GETTABLE:
			// A B C | R(A) := UpValue[B][RK(C)]
			// A B C | R(A) := R(B)[RK(C)]
			// Status: done
			t := (*args.B).(types.Table)
			*args.A = t.Get(*args.C)
			if op == types.OP_GETTABUP {
				fmt.Printf("%-10sR(A)=%v U(B)=%v RK(C)=%v\n", op, *args.A, t, *args.C)
			} else {
				fmt.Printf("%-10sR(A)=%v R(B)=%v RK(C)=%v\n", op, *args.A, t, *args.C)
			}

		case types.OP_SETTABUP, types.OP_SETTABLE:
			// A B C | UpValue[A][RK(B)] := RK(C)
			// A B C | R(A)[RK(B)] := RK(C)
			// Status: done
			t := (*args.A).(types.Table)
			t.Set(*args.B, *args.C)
			if op == types.OP_SETTABUP {
				fmt.Printf("%-10sU(A)=%v RK(B)=%v RK(C)=%v\n", op, t, *args.B, *args.C)
			} else {
				fmt.Printf("%-10sR(A)=%v RK(B)=%v RK(C)=%v\n", op, t, *args.B, *args.C)
			}

		case types.OP_SETUPVAL:
			// A B | UpValue[B] := R(A)
			// Status: untested
			*args.B = *args.A
			fmt.Printf("%-10sR(A)=%v U(B)=%v\n", op, *args.A, *args.B)

		case types.OP_NEWTABLE:
			// A B C | R(A) := {} (size = B,C)
			// Status: incomplete, missing array and hash sizes
			t := types.NewTable()
			// TODO : Encoded array and hash sizes (B and C) are ignored at the moment
			*args.A = t
			fmt.Printf("%-10sR(A)=%v B=%v C=%v\n", op, t, args.Bx, args.Cx)

		case types.OP_SELF:
			// A B C | R(A+1) := R(B); R(A) := R(B)[RK(C)]
			// Status: done
			s.CI.Frame[args.Ax+1] = *args.B
			t := (*args.B).(types.Table)
			s.CI.Frame[args.Ax] = t.Get(*args.C)
			fmt.Printf("%-10sA=%v R(B)=%v RK(C)=%v\n", op, args.Ax, t, *args.C)

		case types.OP_ADD, types.OP_SUB, types.OP_MUL, types.OP_DIV,
			types.OP_MOD, types.OP_POW:
			// A B C | R(A) := RK(B) + RK(C)
			// A B C | R(A) := RK(B) - RK(C)
			// Status: incomplete, missing metamethods
			// A B C | R(A) := RK(B) * RK(C)
			// A B C | R(A) := RK(B) ÷ RK(C)
			// A B C | R(A) := RK(B) % RK(C)
			// A B C | R(A) := RK(B) ^ RK(C)
			*args.A = coerceAndComputeBinaryOp(_BINOPS[op], *args.B, *args.C)
			fmt.Printf("%-10sR(A)=%v RK(B)=%v RK(C)=%v\n", op, *args.A, *args.B, *args.C)

		case types.OP_UNM:
			// A B | R(A) := -R(B)
			// Status: incomplete, missing metamethods
			*args.A = coerceAndComputeUnaryOp('-', *args.B)
			fmt.Printf("%-10sR(A)=%v R(B)=%v\n", op, *args.A, *args.B)

		case types.OP_NOT:
			// A B | R(A) := not R(B)
			*args.A = isFalse(*args.B)
			fmt.Printf("%-10sR(A)=%v R(B)=%v\n", op, *args.A, *args.B)

		case types.OP_LEN:
			// A B | R(A) := length of R(B)
			*args.A = computeLength(*args.B)
			fmt.Printf("%-10sR(A)=%v R(B)=%v\n", op, *args.A, *args.B)

		case types.OP_CONCAT:
			// A B C | R(A) := R(B).. ... ..R(C)
			src := s.CI.Frame[args.Bx : args.Cx+1]
			*args.A = coerceAndConcatenate(src)
			fmt.Printf("%-10sR(A)=%v B=%v C=%v\n", op, *args.A, args.Bx, args.Cx)

		case types.OP_JMP:
			// A sBx | pc+=sBx; if (A) close all upvalues >= R(A) + 1
			doJump(s, args, 0)
			fmt.Printf("%-10sA=%v sBx=%v\n", op, args.Ax, args.Bx)

		case types.OP_EQ, types.OP_LT, types.OP_LE:
			// A B C | if ((RK(B) == RK(C)) ~= A) then pc++
			// A B C | if ((RK(B) <  RK(C)) ~= A) then pc++
			// A B C | if ((RK(B) <= RK(C)) ~= A) then pc++
			if _CMPOPS[op](*args.B, *args.C) != asBool(args.Ax) {
				s.CI.PC++
			} else {
				// For the fall-through case, a JMP is always expected, in order to optimize
				// execution in the virtual machine.
				if i2 := s.CI.Cl.P.Code[s.CI.PC]; i2.GetOpCode() != types.OP_JMP {
					panic(fmt.Sprintf("%s: expected OP_JMP as next instruction, found %s", op, i2.GetOpCode()))
				} else {
					s.OpCodeDebug = append(s.OpCodeDebug, i2.GetOpCode())
					doJump(s, i2.GetArgs(s), 1)
				}
			}
			fmt.Printf("%-10sA=%v RK(B)=%v RK(C)=%v\n", op, args.Ax, *args.B, *args.C)

		case types.OP_TEST:
			// A C | if not (R(A) <=> C) then pc++
			if asBool(args.Cx) == isFalse(*args.A) {
				s.CI.PC++
			} else {
				if i2 := s.CI.Cl.P.Code[s.CI.PC]; i2.GetOpCode() != types.OP_JMP {
					panic(fmt.Sprintf("%s: expected OP_JMP as next instruction, found %s", op, i2.GetOpCode()))
				} else {
					s.OpCodeDebug = append(s.OpCodeDebug, i2.GetOpCode())
					doJump(s, i2.GetArgs(s), 1)
				}
			}
			fmt.Printf("%-10sR(A)=%v C=%v\n", op, *args.A, args.Cx)

		case types.OP_TESTSET:
			// A B C | if (R(B) <=> C) then R(A) := R(B) else pc++
			if asBool(args.Cx) == isFalse(*args.B) {
				s.CI.PC++
			} else {
				*args.A = *args.B
				if i2 := s.CI.Cl.P.Code[s.CI.PC]; i2.GetOpCode() != types.OP_JMP {
					panic(fmt.Sprintf("%s: expected OP_JMP as next instruction, found %s", op, i2.GetOpCode()))
				} else {
					s.OpCodeDebug = append(s.OpCodeDebug, i2.GetOpCode())
					doJump(s, i2.GetArgs(s), 1)
				}
			}
			fmt.Printf("%-10sR(A)=%v R(B)=%v C=%v\n", op, *args.A, *args.B, args.Cx)

		case types.OP_CALL:
			// A B C | R(A), ... ,R(A+C-2) := R(A)(R(A+1), ... ,R(A+B-1))
			// CALL always updates the top of stack value.
			// B=1 means no parameter. B=0 means up-to-top-of-stack parameters. B=2 means 1 parameter, and so on.
			// C=1 means 1 return value. C=0 means multiple return values. C=2 means 1 return value, and so on.
			nRets := args.Cx - 1
			if asBool(args.Bx) {
				// Adjust top of stack, since we know exactly the number of arguments.
				s.Top = s.CI.Base + args.Ax + args.Bx
			}
			// Else, it is because last param to this call was a func call with unknown
			// number of results, so this call actually set the Top to whatever it had to be.
			if preCall(s, args, nRets) {
				// TODO : What to do if Go Func call?
			} else {
				fmt.Printf("%-10sR(A)=%v B=%v C=%v\n", op, *args.A, args.Bx, args.Cx)
				goto newFrame
			}

		case types.OP_TAILCALL:
			panic("TAILCALL: not implemented")

		case types.OP_RETURN:
			// A B | return R(A), ... ,R(A+B-2)
			// TODO : Test coroutines!
			if asBool(args.Bx) {
				s.Top = s.CI.Base + args.Ax + args.Bx - 1
			}
			if len(s.CI.Cl.P.Protos) > 0 {
				closeUpvalues(s, s.CI.Base+args.Ax)
			}
			args.Bx = posCall(s, s.CI.Base+args.Ax)

			if s.CI == nil {
				// TODO : Is this equivalent to Lua's check of CIST_REENTRY?
				fmt.Printf("%s\n", op)
				return
			} else {
				if asBool(args.Bx) {
					// TODO : Set Top back to CI.Top? I don't have a CI.Top! Do I need a CI.Top? Probably!
				}
				if prevOp := s.CI.Cl.P.Code[s.CI.PC-1].GetOpCode(); prevOp != types.OP_CALL {
					panic(fmt.Sprintf("expected CALL to be previous instruction in RETURNed frame, got %s", prevOp))
				}
				fmt.Printf("%-10sR(A)=%v B=%v\n", op, *args.A, args.Bx)
				goto newFrame
			}

		case types.OP_FORLOOP:
			// A sBx | R(A)+=R(A+2); if R(A) <?= R(A+1) then { pc+=sBx; R(A+3)=R(A) }
			step := s.CI.Frame[args.Ax+2].(float64)
			idx := (*args.A).(float64) + step
			limit := s.CI.Frame[args.Ax+1].(float64)

			if (0 < step && idx <= limit) || (0 >= step && limit <= idx) {
				s.CI.PC += args.Bx
				*args.A = idx
				s.CI.Frame[args.Ax+3] = idx
			}
			fmt.Printf("%-10sR(A)=%v sBx=%v\n", op, *args.A, args.Bx)

		case types.OP_FORPREP:
			// A sBx | R(A)-=R(A+2); pc+=sBx
			init, ok := coerceToNumber(*args.A)
			if !ok {
				panic(fmt.Sprintf("%s: initial value must be a number", op))
			}
			_, ok = coerceToNumber(s.CI.Frame[args.Ax+1])
			if !ok {
				panic(fmt.Sprintf("%s: limit must be a number", op))
			}
			step, ok := coerceToNumber(s.CI.Frame[args.Ax+2])
			if !ok {
				panic(fmt.Sprintf("%s: step must be a number", op))
			}
			*args.A = init - step
			s.CI.PC += args.Bx
			fmt.Printf("%-10sR(A)=%v sBx=%v\n", op, *args.A, args.Bx)

		case types.OP_TFORCALL:
			// A C | R(A+3), ... ,R(A+2+C) := R(A)(R(A+1), R(A+2));
			callBase := args.Ax + 3
			s.CI.Frame[callBase+2] = s.CI.Frame[args.Ax+2]
			s.CI.Frame[callBase+1] = s.CI.Frame[args.Ax+1]
			s.CI.Frame[callBase] = s.CI.Frame[args.Ax]
			s.Top = s.CI.Base + callBase + 3 // Func + 2 args (state and index)
			// TODO : luaD_call(s, cb, args.Cx, 1)
			// TODO: s.Top = s.CI.Top

			// Fallthrough to the TFORLOOP, which must always follow a TFORCALL
			i = s.CI.Cl.P.Code[s.CI.PC]
			op = i.GetOpCode()
			s.OpCodeDebug = append(s.OpCodeDebug, op)
			if op != types.OP_TFORLOOP {
				panic(fmt.Sprintf("OP_TFORCALL: expected OP_TFORLOOP as next instruction, found %s", op))
			}
			// Consume instruction
			s.CI.PC++
			args = i.GetArgs(s)
			fmt.Printf("%-10sA=%v C=%v\n", op, args.Ax, args.Cx)
			fallthrough // *** explicit FALLTHROUGH

		case types.OP_TFORLOOP:
			// A sBx | if R(A+1) ~= nil then { R(A)=R(A+1); pc += sBx }
			if !isNil(s.CI.Frame[args.Ax+1]) {
				*args.A = s.CI.Frame[args.Ax+1]
				s.CI.PC += args.Bx
			}
			fmt.Printf("%-10sR(A)=%v sBx=%v\n", op, *args.A, args.Bx)

		case types.OP_SETLIST:
			// A B C | R(A)[(C-1)*FPF+i] := R(A+i), 1 <= i <= B
			// Sets the values for a range of array elements. B is the number of elements.
			// Field C encodes the block number of the table to be initialized. The block
			// size is denoted by FPF. FPF is “fields per flush”, with a value of 50.
			// For example, for array locations 1 to 20, C will be 1 and B will be 20.
			var c, n int
			var t types.Table
			var ok bool

			if n = args.Bx; n == 0 {
				// Determine n using top of the stack
				n = s.Top - (s.CI.Base + args.Ax) - 1
			}
			if c = args.Cx; c == 0 {
				// Use the following EXTRAARG instruction to get the value
				if i2 := s.CI.Cl.P.Code[s.CI.PC]; i2.GetOpCode() != types.OP_EXTRAARG {
					panic(fmt.Sprintf("%s: expected OP_EXTRAARG as next instruction, found %s", op, i2.GetOpCode()))
				} else {
					s.CI.PC++
					s.OpCodeDebug = append(s.OpCodeDebug, i2.GetOpCode())
					c = i2.GetArgAx()
				}
			}
			if t, ok = (*args.A).(types.Table); !ok {
				panic(fmt.Sprintf("%s: expected R(A) to be a Table", op))
			}
			last := ((c - 1) * types.LFIELDS_PER_FLUSH) + n
			// TODO : Can optimize Table's array allocation here
			/*
				if last > t.ArraySize() {
				}
			*/
			// Array portion of Lua's tables are 1-indexed, NOT 0!
			for ; n > 0; n-- {
				t.Set(last, s.CI.Frame[args.Ax+n])
				last--
			}
			// TODO : Damn CI.Top...
			//s.Top = s.CI.Top
			fmt.Printf("%-10sR(A)=%v B=%v C=%v\n", op, *args.A, args.Bx, args.Cx)

		case types.OP_CLOSURE:
			// A Bx | R(A) := closure(KPROTO[Bx])
			p := s.CI.Cl.P.Protos[args.Bx]
			// TODO : Optimize by caching closures, see getcached() in lvm.c
			pushClosure(s, p, args.A)
			fmt.Printf("%-10sR(A)=%v Bx=%v\n", op, *args.A, args.Bx)

		case types.OP_VARARG:
			// A B | R(A), R(A+1), ..., R(A+B-2) = vararg
			b := args.Bx - 1
			n := s.CI.Base - s.CI.FuncIndex - int(s.CI.Cl.P.Meta.NumParams) - 1
			if b < 0 {
				b = n
				s.CheckStack(byte(n))
				s.Top = s.CI.Base + args.Ax + n
			}
			for j := 0; j < b; j++ {
				if j < n {
					s.CI.Frame[args.Ax+j] = s.CI.Frame[s.CI.Base-n+j]
				} else {
					s.CI.Frame[args.Ax+j] = nil
				}
			}
			fmt.Printf("%-10sA=%v B=%v\n", op, args.Ax, args.Bx)

		default:
			panic(fmt.Sprintf("%s: unexpected opcode", op))
		}
	}
}
