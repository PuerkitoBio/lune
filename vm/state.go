package vm

import (
	"fmt"
	"github.com/PuerkitoBio/lune/types"
)

const (
	_INITIAL_STACK_CAPACITY = 2
)

// Holds pointers to values (pointer to empty interface - yes, I know, but it is 
// required because the interface may hold the value inline for numbers and bools):
// http://play.golang.org/p/e2Ptu8puSZ
type Stack struct {
	top int // First free slot
	stk []*types.Value
}

func newStack() *Stack {
	return &Stack{0, make([]*types.Value, _INITIAL_STACK_CAPACITY)}
}

type State struct {
	stack   *Stack
	globals types.Table
}

func (s *Stack) Get(idx int) *types.Value {
	return s.stk[idx]
}

func (s *Stack) push(v types.Value) {
	s.stk[s.top] = &v
	s.top++
}

// TODO : Required?
func (s *Stack) checkStack(needed byte) {
	missing := cap(s.stk) - (s.top + int(needed) + 1) // i.e. cap=10, top=7 and is last used - so 8 slots taken, needed=3: 10-(7 + 3 + 1)
	if missing > 0 {
		dummy := make([]*types.Value, missing)
		s.stk = append(s.stk, dummy...)
	}
}

func (s *Stack) dumpStack() {
	fmt.Println("*** DUMP STACK ***")
	for i, v := range s.stk {
		if v == nil {
			fmt.Println(i, v)
		} else {
			fmt.Println(i, *v)
		}
	}
}

func NewState(entryPoint *types.Prototype) *State {
	s := &State{newStack(), make(types.Table)}

	cl := types.NewClosure(entryPoint)
	if l := len(entryPoint.Upvalues); l == 1 {
		// 1 upvalue = globals table as upvalue
		v := types.Value(s.globals)
		cl.UpVals[0] = &v
	} else if l > 1 {
		// TODO : panic?
		panic("too many upvalues expected for entry point")
	}

	// Push the closure on the stack
	s.stack.push(cl)
	return s
}

type CallInfo struct {
	Cl         *types.Closure
	FuncIndex  int
	NumResults int
	CallStatus byte
	PC         int
	Base       int
}

func newCallInfo(s *State, fIdx int) *CallInfo {
	// Get the function's closure at this stack index
	f := s.stack.Get(fIdx)
	cl := (*f).(*types.Closure)

	// Make sure the stack has enough slots
	s.stack.checkStack(cl.P.Meta.MaxStackSize)

	// Complete the arguments
	n := s.stack.top - fIdx - 1
	for ; n < int(cl.P.Meta.NumParams); n++ {
		s.stack.push(nil)
	}

	ci := new(CallInfo)
	ci.Cl = cl
	ci.FuncIndex = fIdx
	ci.NumResults = 0 // TODO : For now, ignore, someday will be passed
	ci.CallStatus = 0 // TODO : For now, ignore
	ci.PC = 0
	ci.Base = fIdx + 1 // TODO : For now, considre the base to be fIdx + 1, will have to manage varargs someday

	return ci
}

/*
type gState struct {
	strt strTable
}

func newGState() *gState {
	return &gState{newStrTable()}
}

type lState struct {
	g      *gState
	stk    *stack
	ci     *callInfo
	baseCi callInfo
}

func NewState() *lState {
	return &lState{newGState(), newStack(), nil, callInfo{}}
}

type callInfo struct {
	funcStkIdx uint
	prev, next *callInfo
	nResults   uint8
}
*/
