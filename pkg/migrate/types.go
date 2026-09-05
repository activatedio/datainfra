package migrate

import "context"

// Migrator applies a one-directional migration. It is the base tier of the
// test lifecycle and the production bring-up stage: schema and any data a
// database is expected to carry for its whole life.
type Migrator interface {
	Migrate(ctx context.Context) error
}

// Reversible is a migration that can be undone. It is the delta tier of the
// test lifecycle: whatever a single test applies on top of a shared base and
// must leave no trace of afterwards. Down must restore the database to the
// state Up found it in, and must be safe to call after a partially failed Up.
type Reversible interface {
	Up(ctx context.Context) error
	Down(ctx context.Context) error
}

// Func adapts two functions to a Reversible. Either may be nil, in which case
// that direction is a no-op.
type Func struct {
	UpFunc   func(ctx context.Context) error
	DownFunc func(ctx context.Context) error
}

func (f Func) Up(ctx context.Context) error {
	if f.UpFunc == nil {
		return nil
	}
	return f.UpFunc(ctx)
}

func (f Func) Down(ctx context.Context) error {
	if f.DownFunc == nil {
		return nil
	}
	return f.DownFunc(ctx)
}

// UpOnly lifts a Migrator into a Reversible whose Down does nothing. Use it
// to place a one-directional migrator inside a Sequence whose undo is carried
// by a later step (typically a table reset).
func UpOnly(m Migrator) Reversible {
	return Func{UpFunc: m.Migrate}
}

type sequence struct {
	steps []Reversible
}

// Sequence composes Reversibles: Up runs the steps in order, Down runs every
// step's Down in reverse order. Down does not stop at the first error — each
// step gets its chance to undo — and returns the first error seen.
func Sequence(steps ...Reversible) Reversible {
	return &sequence{steps: steps}
}

func (s *sequence) Up(ctx context.Context) error {
	for _, st := range s.steps {
		if err := st.Up(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (s *sequence) Down(ctx context.Context) error {
	var first error
	for i := len(s.steps) - 1; i >= 0; i-- {
		if err := s.steps[i].Down(ctx); err != nil && first == nil {
			first = err
		}
	}
	return first
}
