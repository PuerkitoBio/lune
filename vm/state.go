package vm

import (
	"github.com/PuerkitoBio/lune/serializer"
	"github.com/PuerkitoBio/lune/types"
)

type Stack struct {
	top int // First free slot
	stk []types.Value
}

type State struct {
	stack *Stack
}

func (s *State) Get(idx int) types.Value {
	return s.stack.stk[idx]
}

func (s *State) push(v types.Value) {
	s.stack.stk[s.stack.top] = v
	s.stack.top++
}

func (s *State) checkStack(needed byte) {
	missing := cap(s.stack.stk) - (s.stack.top + int(needed) + 1) // i.e. cap=10, top=7 and is last used - so 8 slots taken, needed=3: 10-(7 + 3 + 1)
	if missing > 0 {
		dummy := make([]types.Value, missing)
		s.stack.stk = append(s.stack.stk, dummy...)
	}
}

func NewState(entryPoint *serializer.Prototype) *State {
	s := &State{new(Stack)}
	s.push(entryPoint)
	return s
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
