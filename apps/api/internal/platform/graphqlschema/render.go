package graphqlschema

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// SDL checks every registration and renders the schema. The output is deterministic: scalars,
// enums, Query, Mutation, then object and input types, each group sorted by name, and fields in
// declaration order followed by the resolver fields in registration order. All problems are
// returned together, joined with errors.Join, each naming the Go type and field.
func (s *Schema) SDL() ([]byte, error) {
	r := &renderer{
		s:       s,
		scalars: map[string]string{},
		enums:   map[string]*enumType{},
		types:   map[string]*objectType{},
		goTypes: map[string]reflect.Type{},
	}
	r.collect()
	if len(r.errs) > 0 {
		return nil, errors.Join(r.errs...)
	}
	return r.render(), nil
}

type renderer struct {
	s       *Schema
	errs    []error
	scalars map[string]string // name to description
	enums   map[string]*enumType
	types   map[string]*objectType
	// goTypes maps each GraphQL type name to the Go type it came from, to catch two Go types that
	// would share one name and make gqlgen's autobind ambiguous.
	goTypes map[string]reflect.Type
}

type objectType struct {
	name, doc string
	input     bool
	fields    []renderedField
}

type renderedField struct {
	name, doc, deprecated, typ, def string
	args                            []renderedField
}

type enumType struct {
	name, doc string
	values    []string
}

var (
	namePattern   = regexp.MustCompile(`^[_A-Za-z][_0-9A-Za-z]*$`)
	noArgs        = reflect.TypeFor[NoArgs]()
	rootTypeNames = []string{"Query", "Mutation"}
)

func (r *renderer) fail(format string, args ...any) {
	r.errs = append(r.errs, fmt.Errorf(format, args...))
}

// collect resolves the root fields, then attaches resolver fields to their parent types until no
// more can be attached; a field whose parent never appears is an error.
func (r *renderer) collect() {
	pending := map[string]bool{}
	for _, parent := range r.s.order {
		if slices.Contains(rootTypeNames, parent) {
			r.types[parent] = &objectType{name: parent}
			r.attach(parent)
			continue
		}
		pending[parent] = true
	}
	for len(pending) > 0 {
		progress := false
		for _, parent := range r.s.order {
			if !pending[parent] {
				continue
			}
			if t, ok := r.types[parent]; ok && !t.input {
				r.attach(parent)
				delete(pending, parent)
				progress = true
			}
		}
		if !progress {
			break
		}
	}
	for _, parent := range r.s.order {
		if pending[parent] {
			for _, f := range r.s.fields[parent] {
				r.fail("field %s.%s: parent type %s is not an object type of the schema", parent, f.name, parent)
			}
		}
	}
	for name, t := range r.types {
		if strings.HasSuffix(name, "Connection") && !t.input {
			var names []string
			for _, f := range t.fields {
				names = append(names, f.name)
			}
			if !slices.Equal(names, []string{"nodes", "pageInfo"}) {
				r.fail("type %s: a connection has exactly the fields nodes and pageInfo, got %v", name, names)
			}
		}
	}
}

// attach renders the registered fields of a parent type and appends them to it.
func (r *renderer) attach(parent string) {
	t := r.types[parent]
	for _, f := range r.s.fields[parent] {
		where := parent + "." + f.name
		rf := renderedField{name: f.name, doc: f.doc, typ: r.ref(f.result, false, where), args: r.args(f.args, where)}
		r.addField(t, rf, where)
	}
}

func (r *renderer) addField(t *objectType, f renderedField, where string) {
	if !namePattern.MatchString(f.name) {
		r.fail("%s: %q is not a valid GraphQL name", where, f.name)
	}
	for _, existing := range t.fields {
		if existing.name == f.name {
			r.fail("%s: type %s already has a field %s", where, t.name, f.name)
			return
		}
	}
	t.fields = append(t.fields, f)
}

// args turns the fields of an Args struct into arguments.
func (r *renderer) args(t reflect.Type, where string) []renderedField {
	if t == noArgs {
		return nil
	}
	if t.Kind() != reflect.Struct {
		r.fail("%s: arguments must be a struct, got %s", where, t)
		return nil
	}
	return r.structFields(t, true, where)
}

// structFields renders the exported fields of a struct, flattening embedded structs the way
// encoding/json does.
func (r *renderer) structFields(t reflect.Type, input bool, where string) []renderedField {
	var out []renderedField
	for i := range t.NumField() {
		f := t.Field(i)
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "-" {
			continue
		}
		if f.Anonymous && name == "" {
			embedded := f.Type
			if embedded.Kind() == reflect.Pointer {
				embedded = embedded.Elem()
			}
			if embedded.Kind() == reflect.Struct {
				out = append(out, r.structFields(embedded, input, where)...)
				continue
			}
		}
		if !f.IsExported() {
			continue
		}
		if name == "" {
			name = f.Name
		}
		fieldWhere := where + "." + f.Name
		rf := renderedField{
			name:       name,
			doc:        f.Tag.Get("doc"),
			deprecated: f.Tag.Get("deprecated"),
			typ:        r.ref(f.Type, input, fieldWhere),
		}
		if def, ok := f.Tag.Lookup("default"); ok {
			if !input {
				r.fail("%s: a default applies to arguments and input fields only", fieldWhere)
			}
			rf.def = r.defaultValue(rf.typ, def)
		}
		out = append(out, rf)
	}
	return out
}

// ref renders the GraphQL type of a Go type: a pointer is nullable, anything else non-null.
func (r *renderer) ref(t reflect.Type, input bool, where string) string {
	nullable := false
	if t.Kind() == reflect.Pointer {
		nullable = true
		t = t.Elem()
	}
	inner := r.named(t, input, where)
	if nullable || inner == "" {
		return inner
	}
	return inner + "!"
}

func (r *renderer) named(t reflect.Type, input bool, where string) string {
	if def, ok := r.s.scalars[t]; ok {
		r.scalars[def.name] = def.doc
		return def.name
	}
	switch t.Kind() {
	case reflect.Slice:
		if t.Elem().Kind() == reflect.Uint8 {
			r.fail("%s: []byte has no GraphQL mapping; register a scalar", where)
			return ""
		}
		return "[" + r.ref(t.Elem(), input, where) + "]"
	case reflect.Struct:
		return r.structType(t, input, where)
	case reflect.String:
		if values, ok := enumValues(t); ok {
			r.enum(t, values, where)
			return t.Name()
		}
		return "String"
	case reflect.Bool:
		return "Boolean"
	case reflect.Int, reflect.Int32, reflect.Int16, reflect.Int8, reflect.Uint16, reflect.Uint8:
		return "Int"
	default:
		r.fail("%s: Go type %s has no GraphQL mapping; money and IDs use a named scalar type, never a number", where, t)
		return ""
	}
}

// enumValues returns the values of a named string type with an EnumValues method.
func enumValues(t reflect.Type) ([]string, bool) {
	if t.Name() == "" {
		return nil, false
	}
	e, ok := reflect.Zero(t).Interface().(interface{ EnumValues() []string })
	if !ok {
		return nil, false
	}
	return e.EnumValues(), true
}

func (r *renderer) enum(t reflect.Type, values []string, where string) {
	if !r.claim(t, where) {
		return
	}
	if _, ok := r.enums[t.Name()]; ok {
		return
	}
	for _, v := range values {
		if !namePattern.MatchString(v) || v == "true" || v == "false" || v == "null" {
			r.fail("%s: enum %s value %q is not a valid GraphQL name", where, t.Name(), v)
		}
	}
	r.enums[t.Name()] = &enumType{name: t.Name(), doc: r.s.docs[t], values: values}
}

// claim records that a GraphQL type name comes from t, and fails when another Go type already
// took it.
func (r *renderer) claim(t reflect.Type, where string) bool {
	name := t.Name()
	if name == "" {
		r.fail("%s: anonymous type %s has no GraphQL name", where, t)
		return false
	}
	if strings.ContainsAny(name, "[]") {
		r.fail("%s: generic type %s has no GraphQL name", where, t)
		return false
	}
	if other, ok := r.goTypes[name]; ok && other != t {
		r.fail("%s: GraphQL name %s is used by both %s and %s", where, name, other, t)
		return false
	}
	r.goTypes[name] = t
	return true
}

func (r *renderer) structType(t reflect.Type, input bool, where string) string {
	if !r.claim(t, where) {
		return ""
	}
	name := t.Name()
	if existing, ok := r.types[name]; ok {
		if existing.input != input {
			r.fail("%s: struct %s is used both as an input and as an output; give the input its own type", where, t)
		}
		return name
	}
	ot := &objectType{name: name, doc: r.s.docs[t], input: input}
	r.types[name] = ot
	for _, f := range r.structFields(t, input, name) {
		r.addField(ot, f, name+"."+f.name)
	}
	return name
}

// defaultValue renders a default in GraphQL syntax: numbers, booleans and enum values bare,
// everything else quoted.
func (r *renderer) defaultValue(typ, value string) string {
	base := strings.Trim(typ, "[]!")
	switch {
	case base == "Int" || base == "Int64" || base == "Boolean":
		return value
	case r.enums[base] != nil:
		return value
	default:
		return strconv.Quote(value)
	}
}

func (r *renderer) render() []byte {
	var b strings.Builder
	for _, name := range sortedKeys(r.scalars) {
		writeDescription(&b, r.scalars[name], "")
		fmt.Fprintf(&b, "scalar %s\n\n", name)
	}
	for _, name := range sortedKeys(r.enums) {
		e := r.enums[name]
		writeDescription(&b, e.doc, "")
		fmt.Fprintf(&b, "enum %s {\n", name)
		for _, v := range e.values {
			fmt.Fprintf(&b, "  %s\n", v)
		}
		b.WriteString("}\n\n")
	}
	var names []string
	for _, name := range rootTypeNames {
		if _, ok := r.types[name]; ok {
			names = append(names, name)
		}
	}
	var objects, inputs []string
	for _, name := range sortedKeys(r.types) {
		switch {
		case slices.Contains(rootTypeNames, name):
		case r.types[name].input:
			inputs = append(inputs, name)
		default:
			objects = append(objects, name)
		}
	}
	names = append(append(names, objects...), inputs...)
	for i, name := range names {
		t := r.types[name]
		writeDescription(&b, t.doc, "")
		keyword := "type"
		if t.input {
			keyword = "input"
		}
		fmt.Fprintf(&b, "%s %s {\n", keyword, name)
		for _, f := range t.fields {
			writeField(&b, f)
		}
		b.WriteString("}\n")
		if i < len(names)-1 {
			b.WriteString("\n")
		}
	}
	return []byte(b.String())
}

func writeField(b *strings.Builder, f renderedField) {
	writeDescription(b, f.doc, "  ")
	b.WriteString("  " + f.name)
	if len(f.args) > 0 {
		multiline := false
		for _, a := range f.args {
			if a.doc != "" {
				multiline = true
			}
		}
		b.WriteString("(")
		for i, a := range f.args {
			if multiline {
				b.WriteString("\n")
				writeDescription(b, a.doc, "    ")
				b.WriteString("    ")
			} else if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(a.name + ": " + a.typ)
			if a.def != "" {
				b.WriteString(" = " + a.def)
			}
		}
		if multiline {
			b.WriteString("\n  ")
		}
		b.WriteString(")")
	}
	b.WriteString(": " + f.typ)
	if f.def != "" {
		b.WriteString(" = " + f.def)
	}
	if f.deprecated != "" {
		fmt.Fprintf(b, " @deprecated(reason: %s)", strconv.Quote(f.deprecated))
	}
	b.WriteString("\n")
}

// writeDescription writes a description: a quoted string on one line, or a block string when
// the text has a line break or a quote.
func writeDescription(b *strings.Builder, doc, indent string) {
	if doc == "" {
		return
	}
	if !strings.ContainsAny(doc, "\n\"\\") {
		fmt.Fprintf(b, "%s\"%s\"\n", indent, doc)
		return
	}
	fmt.Fprintf(b, "%s\"\"\"\n", indent)
	for _, line := range strings.Split(strings.ReplaceAll(doc, `"""`, `\"""`), "\n") {
		if line == "" {
			b.WriteString("\n")
			continue
		}
		fmt.Fprintf(b, "%s%s\n", indent, line)
	}
	fmt.Fprintf(b, "%s\"\"\"\n", indent)
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
