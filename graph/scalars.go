package graph

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type UUID uuid.UUID

func (u UUID) MarshalJSON() ([]byte, error) {
	return json.Marshal(uuid.UUID(u).String())
}

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

func (u UUID) ImplementsGraphQLType(name string) bool {
	return name == "UUID"
}

func UUIDFrom(id uuid.UUID) UUID { return UUID(id) }
func (u UUID) Native() uuid.UUID { return uuid.UUID(u) }

type Time struct {
	T time.Time
}

func (t Time) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.T.UTC().Format(time.RFC3339Nano))
}

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

func (t Time) ImplementsGraphQLType(name string) bool {
	return name == "Time"
}

func TimeFrom(t time.Time) Time { return Time{T: t} }
