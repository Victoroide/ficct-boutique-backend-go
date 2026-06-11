package graph

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// UUID is the GraphQL custom scalar wrapping uuid.UUID. It serializes to its
// canonical string form on the wire and parses incoming string literals.
type UUID uuid.UUID

// MarshalJSON renders the UUID as its canonical string representation.
func (u UUID) MarshalJSON() ([]byte, error) {
	return json.Marshal(uuid.UUID(u).String())
}

// UnmarshalGraphQL parses an incoming string literal into a UUID, returning an
// error if the input is not a string or not a valid UUID.
func (u *UUID) UnmarshalGraphQL(input interface{}) error {
	s, ok := input.(string)
	if !ok {
		return fmt.Errorf("UUID must be a string, got %T", input)
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return fmt.Errorf("invalid UUID: %w", err)
	}
	*u = UUID(id)
	return nil
}

// ImplementsGraphQLType reports whether this scalar implements the named
// GraphQL type, satisfying the graphql-go scalar interface for "UUID".
func (u UUID) ImplementsGraphQLType(name string) bool {
	return name == "UUID"
}

// UUIDFrom wraps a standard uuid.UUID in the GraphQL scalar type.
func UUIDFrom(id uuid.UUID) UUID { return UUID(id) }

// Native unwraps the scalar back to a standard uuid.UUID.
func (u UUID) Native() uuid.UUID { return uuid.UUID(u) }

// Time is the GraphQL custom scalar wrapping a time.Time. It serializes to and
// parses from RFC 3339 (nanosecond precision) in UTC.
type Time struct {
	T time.Time
}

// MarshalJSON renders the time as an RFC 3339 nanosecond string in UTC.
func (t Time) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.T.UTC().Format(time.RFC3339Nano))
}

// UnmarshalGraphQL parses an incoming RFC 3339 nanosecond string into the
// wrapped time.Time, returning an error if the input is not a valid string.
func (t *Time) UnmarshalGraphQL(input interface{}) error {
	s, ok := input.(string)
	if !ok {
		return fmt.Errorf("Time must be a string, got %T", input)
	}
	parsed, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return fmt.Errorf("invalid Time: %w", err)
	}
	t.T = parsed
	return nil
}

// ImplementsGraphQLType reports whether this scalar implements the named
// GraphQL type, satisfying the graphql-go scalar interface for "Time".
func (t Time) ImplementsGraphQLType(name string) bool {
	return name == "Time"
}

// TimeFrom wraps a standard time.Time in the GraphQL scalar type.
func TimeFrom(t time.Time) Time { return Time{T: t} }
