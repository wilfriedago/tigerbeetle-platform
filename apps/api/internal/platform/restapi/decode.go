package restapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/danielgtaylor/huma/v2"
)

// jsonFormat is Huma's JSON format with a decoder whose errors a client can act on.
var jsonFormat = huma.Format{Marshal: huma.DefaultJSONFormat.Marshal, Unmarshal: decodeJSON}

// decodeJSON decodes a body like json.Unmarshal, but words its errors for the client: encoding/json
// names Go types and struct fields ("cannot unmarshal string into Go struct field
// createTransactionBody.postings of type []dto.Posting"), which describe the service rather than
// the request. A type mismatch becomes "expected array, got string" at the JSON Pointer of the
// member. Errors of the scalars' own decoders are written by this service and pass unchanged.
func decodeJSON(data []byte, v any) error {
	err := json.Unmarshal(data, v)
	var typeErr *json.UnmarshalTypeError
	var syntaxErr *json.SyntaxError
	switch {
	case err == nil:
		return nil
	case errors.As(err, &typeErr):
		return &bodyError{pointer: pointerOf(typeErr.Field), detail: fmt.Sprintf("expected %s, got %s", jsonKind(typeErr.Type), typeErr.Value)}
	case errors.As(err, &syntaxErr):
		return &bodyError{pointer: "#", detail: fmt.Sprintf("malformed JSON at byte %d: %s", syntaxErr.Offset, syntaxErr.Error())}
	}
	return err
}

// bodyError is a body decoding error with the JSON Pointer of the member at fault. Huma keeps only
// the text of the error, so the pointer travels in it as "<pointer>: <detail>" and
// splitBodyMessage takes it back out.
type bodyError struct {
	pointer string
	detail  string
}

func (e *bodyError) Error() string {
	return e.pointer + ": " + e.detail
}

// splitBodyMessage returns the pointer and detail of a body error message: those of a bodyError,
// or "#" (the whole body) and the message as it is.
func splitBodyMessage(msg string) (pointer, detail string) {
	if strings.HasPrefix(msg, "#") {
		if p, d, ok := strings.Cut(msg, ": "); ok {
			return p, d
		}
	}
	return "#", msg
}

// pointerOf turns the field path of a json.UnmarshalTypeError into a JSON Pointer in URI fragment
// form. encoding/json joins the members with dots and escapes each one as a pointer token
// already, so "postings.0.amount" becomes "#/postings/0/amount".
func pointerOf(field string) string {
	if field == "" {
		return "#"
	}
	return "#/" + strings.ReplaceAll(field, ".", "/")
}

// jsonKind names the JSON type a Go type decodes from.
func jsonKind(t reflect.Type) string {
	if t == nil {
		return "a value"
	}
	switch t.Kind() {
	case reflect.Bool:
		return "boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "integer"
	case reflect.Float32, reflect.Float64:
		return "number"
	case reflect.String:
		return "string"
	case reflect.Slice, reflect.Array:
		return "array"
	case reflect.Map, reflect.Struct:
		return "object"
	case reflect.Pointer:
		return jsonKind(t.Elem())
	default:
		return "a value"
	}
}
