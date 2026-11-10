package schema

import "github.com/google/uuid"

// uuidV7 returns a time-ordered UUIDv7. Ent's UUID builder only accepts
// defaults of the form func() uuid.UUID, so the error is handled here.
func uuidV7() uuid.UUID {
	id, err := uuid.NewV7()
	if err != nil {
		panic(err)
	}
	return id
}
