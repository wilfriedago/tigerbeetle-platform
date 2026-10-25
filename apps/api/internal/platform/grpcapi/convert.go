// Package grpcapi holds what every gRPC service shares: the generated protobuf code in platformv1
// (buf generate, never edited) and the conversions between it and the wire types of the other
// transports. The services themselves live in each module's interfaces/grpc/v1.
package grpcapi

import (
	"strings"
	"time"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Timestamp converts a time; the zero time is unset.
func Timestamp(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

// OptionalTimestamp converts an optional time.
func OptionalTimestamp(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return Timestamp(*t)
}

// TimeOf converts an optional timestamp from a request; unset gives nil.
func TimeOf(ts *timestamppb.Timestamp) *time.Time {
	if ts == nil {
		return nil
	}
	t := ts.AsTime()
	return &t
}

// Enum is a generated protobuf enum.
type Enum interface {
	~int32
	Descriptor() protoreflect.EnumDescriptor
}

// EnumOf returns the protobuf value of a wire enum value: "open" gives HOLD_STATUS_OPEN. The wire
// values are the REST and GraphQL ones; the protobuf names add the enum's prefix (buf STANDARD).
// An unknown value gives the UNSPECIFIED zero.
func EnumOf[E Enum](value string) E {
	var zero E
	if value == "" {
		return zero
	}
	values := zero.Descriptor().Values()
	v := values.ByName(protoreflect.Name(prefixOf(zero) + strings.ToUpper(value)))
	if v == nil {
		return zero
	}
	return E(v.Number())
}

// ValueOf returns the wire value of a protobuf enum value: HOLD_STATUS_OPEN gives "open", and the
// UNSPECIFIED zero gives "".
func ValueOf[E Enum](e E) string {
	if e == 0 {
		return ""
	}
	v := e.Descriptor().Values().ByNumber(protoreflect.EnumNumber(e))
	if v == nil {
		return ""
	}
	return strings.ToLower(strings.TrimPrefix(string(v.Name()), prefixOf(e)))
}

// prefixOf returns the prefix every value of the enum carries, read from its zero value
// (HOLD_STATUS_UNSPECIFIED gives HOLD_STATUS_).
func prefixOf[E Enum](e E) string {
	zero := e.Descriptor().Values().ByNumber(0)
	return strings.TrimSuffix(string(zero.Name()), "UNSPECIFIED")
}

// Optional returns nil for an empty string, for optional proto3 string fields.
func Optional[S ~string](s S) *string {
	if s == "" {
		return nil
	}
	v := string(s)
	return &v
}

// PageSize converts page_size to the limit the use cases take; zero lets them pick the default.
func PageSize(n int32) int {
	return int(n)
}
