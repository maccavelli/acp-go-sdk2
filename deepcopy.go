package acp

import "reflect"

// deepCopy returns a copy of v that shares no pointer, slice, map or
// interface value with it: each is copied element by element, a struct field
// by field. The timelines keep and return their entities through it, so an
// entity shares nothing with the update it came from or the caller it goes to
// (0049-PLAN-surface-fixes.md, phase 5, S4). An unexported field, which
// reflection cannot set, is kept as it is; every field the timelines reach is
// exported, which TestDeepCopySharesNothing checks. v must hold no cycle.
func deepCopy[T any](v T) T {
	var c T
	reflect.ValueOf(&c).Elem().Set(copyValue(reflect.ValueOf(&v).Elem()))
	return c
}

// copyValue is v copied as deepCopy copies.
func copyValue(v reflect.Value) reflect.Value {
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}
		c := reflect.New(v.Type().Elem())
		c.Elem().Set(copyValue(v.Elem()))
		return c
	case reflect.Slice:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}
		c := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		copyElems(c, v)
		return c
	case reflect.Array:
		c := reflect.New(v.Type()).Elem()
		copyElems(c, v)
		return c
	case reflect.Map:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}
		c := reflect.MakeMapWithSize(v.Type(), v.Len())
		for it := v.MapRange(); it.Next(); {
			c.SetMapIndex(copyValue(it.Key()), copyValue(it.Value()))
		}
		return c
	case reflect.Interface:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}
		c := reflect.New(v.Type()).Elem()
		c.Set(copyValue(v.Elem()))
		return c
	case reflect.Struct:
		c := reflect.New(v.Type()).Elem()
		c.Set(v)
		for i := range v.NumField() {
			if v.Type().Field(i).IsExported() {
				c.Field(i).Set(copyValue(v.Field(i)))
			}
		}
		return c
	default:
		// A boolean, number or string is its own copy.
		return v
	}
}

// copyElems copies v's elements into c, of the same length: at once when they
// hold no reference, one by one otherwise.
func copyElems(c, v reflect.Value) {
	if plainKind(v.Type().Elem().Kind()) {
		reflect.Copy(c, v)
		return
	}
	for i := range v.Len() {
		c.Index(i).Set(copyValue(v.Index(i)))
	}
}

// plainKind reports whether a value of kind k holds no reference.
func plainKind(k reflect.Kind) bool {
	return (k >= reflect.Bool && k <= reflect.Complex128) || k == reflect.String
}
