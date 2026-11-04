package acp

import (
	"reflect"
	"testing"
)

// filler sets every field of a value it reaches: a pointer to a filled
// value, a slice and a map of one filled element, an interface to a map
// holding a slice, a scalar to a value not its zero. It counts the pointers,
// slices and maps it makes, and fails on an unexported field, which deepCopy
// cannot copy.
type filler struct {
	t    *testing.T
	refs int
}

// fillDepth bounds the filler, which fails a type deeper than it, such as a
// recursive one, rather than leave its fields unset.
const fillDepth = 12

func (f *filler) fill(v reflect.Value, path string, depth int) {
	if depth > fillDepth {
		f.t.Errorf("%s: deeper than %d, a recursive type the filler cannot fill", path, fillDepth)
		return
	}
	switch v.Kind() {
	case reflect.Pointer:
		v.Set(reflect.New(v.Type().Elem()))
		f.refs++
		f.fill(v.Elem(), path, depth+1)
	case reflect.Slice:
		v.Set(reflect.MakeSlice(v.Type(), 1, 1))
		f.refs++
		f.fill(v.Index(0), path+"[0]", depth+1)
	case reflect.Map:
		v.Set(reflect.MakeMapWithSize(v.Type(), 1))
		f.refs++
		k, e := reflect.New(v.Type().Key()).Elem(), reflect.New(v.Type().Elem()).Elem()
		f.fill(k, path+"[key]", depth+1)
		f.fill(e, path+"[value]", depth+1)
		v.SetMapIndex(k, e)
	case reflect.Interface:
		// As _meta holds a nested object: a map and a slice, two references.
		v.Set(reflect.ValueOf(map[string]any{"n": []any{"v"}}))
		f.refs += 2
	case reflect.Struct:
		for i := range v.NumField() {
			sf := v.Type().Field(i)
			if !sf.IsExported() {
				f.t.Errorf("%s.%s: unexported, which deepCopy keeps as it is", path, sf.Name)
				continue
			}
			f.fill(v.Field(i), path+"."+sf.Name, depth+1)
		}
	case reflect.Array:
		for i := range v.Len() {
			f.fill(v.Index(i), path, depth+1)
		}
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(1)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(1)
	case reflect.Float32, reflect.Float64:
		v.SetFloat(1.5)
	case reflect.String:
		v.SetString("s")
	default:
		f.t.Errorf("%s: a %s, which the filler does not fill", path, v.Kind())
	}
}

// shared walks a and its copy b together, failing where they share a
// pointer, slice or map, and returns how many such values it compared.
func shared(t *testing.T, a, b reflect.Value, path string) int {
	t.Helper()
	switch a.Kind() {
	case reflect.Pointer, reflect.Map:
		if a.IsNil() {
			return 0
		}
		n := 1
		if a.Pointer() == b.Pointer() {
			t.Errorf("%s: the copy shares the original's %s", path, a.Kind())
		}
		if a.Kind() == reflect.Pointer {
			return n + shared(t, a.Elem(), b.Elem(), path)
		}
		for it := a.MapRange(); it.Next(); {
			n += shared(t, it.Value(), b.MapIndex(it.Key()), path+"[value]")
		}
		return n
	case reflect.Slice:
		if a.IsNil() {
			return 0
		}
		n := 1
		if a.Len() > 0 && a.Pointer() == b.Pointer() {
			t.Errorf("%s: the copy shares the original's slice", path)
		}
		for i := range a.Len() {
			n += shared(t, a.Index(i), b.Index(i), path+"[i]")
		}
		return n
	case reflect.Interface:
		if a.IsNil() {
			return 0
		}
		return shared(t, a.Elem(), b.Elem(), path)
	case reflect.Struct:
		n := 0
		for i := range a.NumField() {
			if a.Type().Field(i).IsExported() {
				n += shared(t, a.Field(i), b.Field(i), path+"."+a.Type().Field(i).Name)
			}
		}
		return n
	case reflect.Array:
		n := 0
		for i := range a.Len() {
			n += shared(t, a.Index(i), b.Index(i), path)
		}
		return n
	default:
		return 0
	}
}

// checkDeepCopy fills a T, copies it, and requires the copy to equal it and
// to share none of the references the filler made.
func checkDeepCopy[T any](t *testing.T) {
	t.Helper()
	var e T
	v := reflect.ValueOf(&e).Elem()
	name := v.Type().Name()
	f := &filler{t: t}
	f.fill(v, name, 0)
	c := deepCopy(e)
	if !reflect.DeepEqual(e, c) {
		t.Errorf("the copy differs from the original:\n got %+v\nwant %+v", c, e)
	}
	if n := shared(t, v, reflect.ValueOf(c), name); n != f.refs || n < 50 {
		t.Errorf("compared %d references, want the %d the filler made, at least 50", n, f.refs)
	}
}

// TestDeepCopySharesNothing requires deepCopy of each entity the timelines
// keep, every field set through each union variant, to equal the original
// and to share no pointer, slice or map with it; a field the schema adds is
// covered without editing the test (0049-PLAN-surface-fixes.md, phase 5,
// S4).
func TestDeepCopySharesNothing(t *testing.T) {
	t.Parallel()
	t.Run("ToolCallEntity", func(t *testing.T) {
		t.Parallel()
		checkDeepCopy[ToolCallEntity](t)
	})
	t.Run("CompactionEntity", func(t *testing.T) {
		t.Parallel()
		checkDeepCopy[CompactionEntity](t)
	})
	t.Run("nil and empty", func(t *testing.T) {
		t.Parallel()
		if c := deepCopy([]ContentBlock(nil)); c != nil {
			t.Errorf("a nil slice copied as %#v, want nil", c)
		}
		if c := deepCopy([]ContentBlock{}); c == nil || len(c) != 0 {
			t.Errorf("an empty slice copied as %#v, want empty and not nil", c)
		}
		if c := deepCopy(map[string]any(nil)); c != nil {
			t.Errorf("a nil map copied as %#v, want nil", c)
		}
		if c := deepCopy[any](nil); c != nil {
			t.Errorf("a nil interface copied as %#v, want nil", c)
		}
	})
}

// TestTimelinesCopyWhatTheyApply requires a timeline to keep a copy of what
// it takes from an update: changing the update after Apply, through a
// pointer, changes nothing Get returns (0049-PLAN-surface-fixes.md, phase 5,
// deviation 5).
func TestTimelinesCopyWhatTheyApply(t *testing.T) {
	t.Parallel()
	var tl ToolCallTimeline
	line := uint32(3)
	u := StartToolCall("t1", "read", WithStartContent([]ToolCallContent{ToolContent(TextBlock("before"))}),
		WithStartLocations([]ToolCallLocation{{Path: "/work/a.go", Line: &line}}))
	tl.Apply(u)
	u.ToolCall.Content[0].Content.Content.Text.Text = "after"
	line = 99
	if e, _ := tl.Get("t1"); e.Content[0].Content.Content.Text.Text != "before" || *e.Locations[0].Line != 3 {
		t.Errorf("tool call timeline state changed through the applied tool_call: %q, line %d", e.Content[0].Content.Content.Text.Text,
			*e.Locations[0].Line)
	}
	upd := UpdateToolCall("t1", WithUpdateContent([]ToolCallContent{ToolContent(TextBlock("updated"))}))
	tl.Apply(upd)
	upd.ToolCallUpdate.Content[0].Content.Content.Text.Text = "after"
	if e, _ := tl.Get("t1"); e.Content[0].Content.Content.Text.Text != "updated" {
		t.Errorf("tool call timeline state changed through the applied tool_call_update: %q", e.Content[0].Content.Content.Text.Text)
	}

	var ct CompactionTimeline
	summary := []ContentBlock{TextBlock("before")}
	meta := map[string]any{"k": map[string]any{"n": 1}}
	ct.Apply(SessionUpdate{CompactionUpdate: &SessionCompactionUpdate{CompactionId: "c1", Status: CompactionStatusInProgress,
		Summary: PatchValue(summary), Meta: PatchValue(meta)}})
	chunk := SessionUpdate{CompactionSummaryChunk: &SessionUpdateCompactionSummaryChunk{CompactionId: "c1", Content: TextBlock("chunk")}}
	ct.Apply(chunk)
	summary[0].Text.Text = "after"
	meta["k"].(map[string]any)["n"] = "after"
	chunk.CompactionSummaryChunk.Content.Text.Text = "after"
	e, _ := ct.Get("c1")
	if e.Summary[0].Text.Text != "before" || e.Summary[1].Text.Text != "chunk" || e.Meta["k"].(map[string]any)["n"] != 1 {
		t.Errorf("compaction timeline state changed through the applied update: %+v", e)
	}
}
