package outer

import (
	"context"
	"errors"

	"github.com/r3labs/diff/v3"
	"github.com/rs/zerolog/log"
	"go.uber.org/fx"
)

// InnerName is the fx name the flavor's repositories are registered under
// when an outer chain wraps them (data.WithRepositoryName). The chain takes
// the inner repository by this name and provides the interface unnamed, so a
// consumer asking for the interface gets the chain.
const InnerName = "inner"

// MethodMetadata describes one repository call to a sink.
type MethodMetadata struct {
	// Source is the entity the repository is over, in snake_case —
	// "rate_schedule" — which is what an audit row records as its type.
	Source string
	// MethodName is the repository method: Create, Update, Delete,
	// DeleteEntity.
	MethodName string
	// TargetID is the key of the row the method acted on, formatted with
	// fmt.Sprint. Filled by the diff proxy, which is the only layer that
	// knows it: an update's changelog does not mention the key precisely
	// because the key did not change.
	TargetID string
	// Redact names the entity's fields tagged `audit:"redact"`. The
	// generator resolves them, since it knows the type and a sink does not;
	// a sink blanks their values and keeps the fact that they changed.
	Redact []string
}

// Change is one field that differs between two versions of an entity.
type Change struct {
	// Type is "create", "update" or "delete".
	Type string `json:"type"`
	// Path is the field's path from the entity, by Go field name.
	Path []string `json:"path"`
	From any      `json:"from"`
	To   any      `json:"to"`
}

// Changelog is every field that differs between two versions of an entity.
type Changelog []Change

// Diff is the changelog from one version of an entity to another. A nil from
// is a create and a nil to is a delete; both nil is an error.
func Diff(from, to any) (Changelog, error) {
	if from == nil && to == nil {
		return nil, errors.New("outer: diff of nothing against nothing")
	}
	cl, err := diff.Diff(from, to)
	if err != nil {
		return nil, err
	}
	out := make(Changelog, 0, len(cl))
	for _, c := range cl {
		out = append(out, Change{Type: c.Type, Path: c.Path, From: c.From, To: c.To})
	}
	return out, nil
}

// Redacted is the changelog with the named fields' values blanked. The
// change stays, so the record still says the field was replaced.
func (c Changelog) Redacted(fields []string) Changelog {
	if len(fields) == 0 {
		return c
	}
	redact := make(map[string]bool, len(fields))
	for _, f := range fields {
		redact[f] = true
	}
	out := make(Changelog, len(c))
	for i, ch := range c {
		if len(ch.Path) > 0 && redact[ch.Path[0]] {
			ch.From, ch.To = redactedValue(ch.From), redactedValue(ch.To)
		}
		out[i] = ch
	}
	return out
}

func redactedValue(v any) any {
	if v == nil {
		return nil
	}
	return "[redacted]"
}

// DiffSink receives the changelog of every write through a diff proxy. It
// runs in the write's context, so a sink writing to the same database joins
// the write's transaction, and an error from it fails the write.
type DiffSink interface {
	Sink(ctx context.Context, m MethodMetadata, changelog Changelog) error
}

type defaultDiffSink struct{}

// Sink logs the change at debug level.
func (defaultDiffSink) Sink(_ context.Context, m MethodMetadata, changelog Changelog) error {
	log.Debug().Str("source", m.Source).Str("method", m.MethodName).Str("target", m.TargetID).
		Interface("change", changelog.Redacted(m.Redact)).Msg("change")
	return nil
}

// DefaultDiffSink is the sink a diff proxy uses when the graph provides
// none: it logs, at debug, so an assembly without an audit store does not
// log every entity it writes.
func DefaultDiffSink() DiffSink {
	return defaultDiffSink{}
}

// SinkOrDefault is s, or DefaultDiffSink if the graph provides none.
func SinkOrDefault(s DiffSink) DiffSink {
	if s == nil {
		return DefaultDiffSink()
	}
	return s
}

// TagInner registers a repository constructor under InnerName, for wiring by
// hand what data.WithRepositoryName does in a generated index.
func TagInner(ctor any) any {
	return fx.Annotate(ctor, fx.ResultTags(`name:"`+InnerName+`"`))
}
