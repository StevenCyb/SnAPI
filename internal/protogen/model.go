package protogen

import (
	"path/filepath"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// ProtoFile is the DTO-relevant content of a single .proto file: every
// message (nested messages flattened to top-level Go structs) and enum it
// declares. One ProtoFile renders to one dedicated file in api/model.
type ProtoFile struct {
	Name      string // base file name without extension, e.g. "todo"
	NeedsTime bool
	Messages  []Message
	Enums     []Enum
}

// Message is a single generated DTO struct.
type Message struct {
	Name   string
	Fields []Field
}

// Field is a single DTO struct field.
type Field struct {
	GoName     string
	JSONName   string
	GoType     string
	OneofGroup string // non-empty names a real (non-synthetic) oneof this field belongs to
}

// Enum is a generated int32-backed enum type plus its named values.
type Enum struct {
	Name   string
	Values []EnumValue
}

// EnumValue is a single `<EnumName>_<VALUE>` constant.
type EnumValue struct {
	Name   string
	Number int32
}

// buildProtoFile converts a compiled file descriptor into the DTO IR.
func buildProtoFile(fd protoreflect.FileDescriptor) ProtoFile {
	pf := ProtoFile{Name: fileBaseName(fd.Path())}

	for i := 0; i < fd.Enums().Len(); i++ {
		pf.Enums = append(pf.Enums, buildEnum(fd.Enums().Get(i)))
	}
	for i := 0; i < fd.Messages().Len(); i++ {
		collectMessage(fd.Messages().Get(i), &pf)
	}

	for _, msg := range pf.Messages {
		for _, f := range msg.Fields {
			if strings.Contains(f.GoType, "time.Time") || strings.Contains(f.GoType, "time.Duration") {
				pf.NeedsTime = true
			}
		}
	}
	return pf
}

// collectMessage flattens md and its nested messages/enums into pf. Map
// entry messages (the synthetic message proto generates for `map<K,V>`
// fields) are skipped -- they never need their own DTO.
func collectMessage(md protoreflect.MessageDescriptor, pf *ProtoFile) {
	if md.IsMapEntry() {
		return
	}

	name := flattenedMessageName(md)
	msg := Message{Name: name}

	realOneofs := make(map[protoreflect.FieldNumber]string, md.Oneofs().Len())
	for i := 0; i < md.Oneofs().Len(); i++ {
		od := md.Oneofs().Get(i)
		if od.IsSynthetic() {
			continue
		}
		for j := 0; j < od.Fields().Len(); j++ {
			realOneofs[od.Fields().Get(j).Number()] = string(od.Name())
		}
	}

	fields := md.Fields()
	for i := 0; i < fields.Len(); i++ {
		f := fields.Get(i)
		msg.Fields = append(msg.Fields, Field{
			GoName:     goFieldName(f.Name()),
			JSONName:   f.JSONName(),
			GoType:     goFieldType(f),
			OneofGroup: realOneofs[f.Number()],
		})
	}
	pf.Messages = append(pf.Messages, msg)

	for i := 0; i < md.Enums().Len(); i++ {
		pf.Enums = append(pf.Enums, buildEnum(md.Enums().Get(i)))
	}
	for i := 0; i < md.Messages().Len(); i++ {
		collectMessage(md.Messages().Get(i), pf)
	}
}

func buildEnum(ed protoreflect.EnumDescriptor) Enum {
	name := flattenedEnumName(ed)
	e := Enum{Name: name}
	values := ed.Values()
	for i := 0; i < values.Len(); i++ {
		v := values.Get(i)
		e.Values = append(e.Values, EnumValue{
			Name:   name + "_" + string(v.Name()),
			Number: int32(v.Number()),
		})
	}
	return e
}

// flattenedMessageName walks up md's parent chain, joining nested message
// names with "_" (protoc-gen-go style) since Go has no nested named types.
func flattenedMessageName(md protoreflect.MessageDescriptor) string {
	name := string(md.Name())
	if parent, ok := md.Parent().(protoreflect.MessageDescriptor); ok {
		return flattenedMessageName(parent) + "_" + name
	}
	return name
}

func flattenedEnumName(ed protoreflect.EnumDescriptor) string {
	if parent, ok := ed.Parent().(protoreflect.MessageDescriptor); ok {
		return flattenedMessageName(parent) + "_" + string(ed.Name())
	}
	return string(ed.Name())
}

// goFieldName converts a proto field name (snake_case) to a Go field name
// (PascalCase). Simple segment-capitalization -- not protoc-gen-go's fuller
// initialism handling (e.g. "id" stays "Id", not "ID").
func goFieldName(name protoreflect.Name) string {
	parts := strings.Split(string(name), "_")
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]))
		b.WriteString(p[1:])
	}
	return b.String()
}

// goFieldType returns the Go type for a field, including `[]`/`map[K]V`
// wrapping for repeated/map fields.
func goFieldType(f protoreflect.FieldDescriptor) string {
	if f.IsMap() {
		return "map[" + scalarGoType(f.MapKey().Kind()) + "]" + singleGoType(f.MapValue()) + ""
	}
	t := singleGoType(f)
	if f.IsList() {
		return "[]" + t
	}
	return t
}

func singleGoType(f protoreflect.FieldDescriptor) string {
	switch f.Kind() {
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return messageFieldType(f.Message())
	case protoreflect.EnumKind:
		return flattenedEnumName(f.Enum())
	default:
		t := scalarGoType(f.Kind())
		if f.HasOptionalKeyword() {
			return "*" + t
		}
		return t
	}
}

// messageFieldType maps a message-kind field to a Go type, special-casing
// the handful of google.protobuf well-known types that have an obvious,
// useful Go equivalent. Anything else is a pointer to the generated DTO
// struct for that message.
func messageFieldType(md protoreflect.MessageDescriptor) string {
	switch string(md.FullName()) {
	case "google.protobuf.Timestamp":
		return "*time.Time"
	case "google.protobuf.Duration":
		// ponytail: JSON round-trips as a plain number of nanoseconds, not
		// protobuf's canonical `"3.5s"` string -- refine if you need that.
		return "*time.Duration"
	case "google.protobuf.Empty":
		return "*struct{}"
	case "google.protobuf.StringValue":
		return "*string"
	case "google.protobuf.BytesValue":
		return "*[]byte"
	case "google.protobuf.BoolValue":
		return "*bool"
	case "google.protobuf.Int32Value":
		return "*int32"
	case "google.protobuf.Int64Value":
		return "*int64"
	case "google.protobuf.UInt32Value":
		return "*uint32"
	case "google.protobuf.UInt64Value":
		return "*uint64"
	case "google.protobuf.FloatValue":
		return "*float32"
	case "google.protobuf.DoubleValue":
		return "*float64"
	case "google.protobuf.Any", "google.protobuf.Struct", "google.protobuf.Value":
		// ponytail: generic fallback, refine with a concrete type if you need typed access.
		return "map[string]interface{}"
	case "google.protobuf.ListValue":
		return "[]interface{}"
	case "google.protobuf.FieldMask":
		return "[]string"
	default:
		return "*" + flattenedMessageName(md)
	}
}

func scalarGoType(k protoreflect.Kind) string {
	switch k {
	case protoreflect.DoubleKind:
		return "float64"
	case protoreflect.FloatKind:
		return "float32"
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return "int32"
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return "int64"
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return "uint32"
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return "uint64"
	case protoreflect.BoolKind:
		return "bool"
	case protoreflect.StringKind:
		return "string"
	case protoreflect.BytesKind:
		return "[]byte"
	default:
		return "interface{}"
	}
}

// topLevelTypeRef returns the bare (unqualified, un-pointered) model type
// name for a message used directly as an RPC's input or output, e.g.
// "GetTodoRequest". google.protobuf.Empty maps to isEmpty=true (no DTO, no
// body). Any other well-known type used directly as RPC input/output is out
// of scope -- define a message wrapping it instead.
func topLevelTypeRef(md protoreflect.MessageDescriptor) (goType string, isEmpty bool) {
	if string(md.FullName()) == "google.protobuf.Empty" {
		return "", true
	}
	return flattenedMessageName(md), false
}

func fileBaseName(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}
