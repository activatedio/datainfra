package migrate

import "context"

// Migrator applies a one-directional migration. It is the production
// bring-up stage: what a deployed database is brought to, with no notion of
// undoing it.
type Migrator interface {
	Migrate(ctx context.Context) error
}

// Layer is one migration unit in a test fixture's stack, with an exact
// reverse. Layers stack in a fixed order declared by the fixture — schema,
// then seed data, then a bootstrap — and a test names the layers it needs.
//
// Down must reverse exactly what Up did: a schema layer drops the tables it
// created, a data layer deletes the rows it inserted, by key. Nothing here
// is inferred from the database; each author states the reverse of their
// own Up. A layer with no Down cannot be a Layer.
type Layer interface {
	Name() string
	Up(ctx context.Context) error
	Down(ctx context.Context) error
}

// Resettable is the optional optimization a Layer may offer. Reset returns
// the database to "this layer and everything below it are freshly applied,
// nothing above is applied" — the same state Down-then-Up of this layer and
// everything above would produce, without the DDL. The fixture then
// re-applies the layers above.
//
// It is an optimization and only that: the fixture falls back to Down and
// Up when a layer does not offer it. A schema layer typically implements it
// as one TRUNCATE over an authored list of its tables.
type Resettable interface {
	Reset(ctx context.Context) error
}

// Keyed is the optional fingerprint of a parameterized Layer. Two instances
// of the same layer name with different keys are different layers to the
// fixture: a bootstrap that seeds test-specific data reports a key derived
// from that data, so a test that needs a different seed gets the old one
// reversed and its own applied.
type Keyed interface {
	Key() string
}

// KeyOf returns the layer's key, or "" for a layer without parameters.
func KeyOf(l Layer) string {
	if k, ok := l.(Keyed); ok {
		return k.Key()
	}
	return ""
}
