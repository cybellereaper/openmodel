package runtime

import "fmt"

type Environment struct {
	parent  *Environment
	values  map[string]Value
	mutable map[string]bool
}

func NewEnvironment(parent *Environment) *Environment {
	return &Environment{
		parent:  parent,
		values:  map[string]Value{},
		mutable: map[string]bool{},
	}
}

func (e *Environment) Define(name string, value Value, mutable bool) {
	e.values[name] = value
	e.mutable[name] = mutable
}

func (e *Environment) Get(name string) (Value, bool) {
	if v, ok := e.values[name]; ok {
		return v, true
	}
	if e.parent != nil {
		return e.parent.Get(name)
	}
	return Null(), false
}

func (e *Environment) IsDefinedHere(name string) bool {
	_, ok := e.values[name]
	return ok
}

func (e *Environment) Assign(name string, value Value) error {
	if _, ok := e.values[name]; ok {
		if !e.mutable[name] {
			return fmt.Errorf("cannot reassign immutable variable %q", name)
		}
		e.values[name] = value
		return nil
	}
	if e.parent != nil {
		return e.parent.Assign(name, value)
	}
	return fmt.Errorf("unknown variable %q", name)
}

func (e *Environment) IsMutable(name string) bool {
	if _, ok := e.values[name]; ok {
		return e.mutable[name]
	}
	if e.parent != nil {
		return e.parent.IsMutable(name)
	}
	return false
}
