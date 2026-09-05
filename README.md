> ## Datainfra
>
> This is a library to enable generation of data infrastructure
>

[![GitHub Workflow Status](https://img.shields.io/github/actions/workflow/status/activatedio/datainfra/ci.yaml?branch=main&style=flat-square)](https://github.com/activatedio/datainfra/actions?query=workflow%3ACI)
[![Go Report Card](https://goreportcard.com/badge/github.com/activatedio/datainfra?style=flat-square)](https://goreportcard.com/report/github.com/activatedio/datainfra)

# Datainfra

Simple project to generate code for repository data access

## Structure

* `genlib` - contains code go generate other code
* `pkg` - runtime support for classes

## Test lifecycle

`pkg/data/testing` separates four things that fixtures usually conflate.

**Store.** One database. Created by Setup, dropped by Teardown. A profile has
one shared store, plus a dedicated one for any test that asks. The store
records which layers it carries and whether a test has run against it since.

**Layer.** One migration unit with an exact reverse (`migrate.Layer`: `Name`,
`Up`, `Down`). Layers stack in the order the fixture declares — schema, then
seed data, then a bootstrap. Every layer's `Down` reverses exactly what its
`Up` did; nothing is inferred from the database. Two optional interfaces:

* `migrate.Resettable` — `Reset` returns the store to "this layer and
  everything below it freshly applied, nothing above" without the DDL. An
  optimization the planner uses when it can; a schema layer typically
  implements it as one `TRUNCATE` over an authored table list.
* `migrate.Keyed` — a fingerprint for a parameterized layer, so a bootstrap
  seeded from one test's data is a different layer from the same bootstrap
  seeded from another's.

`gormmigrate.NewGooseLayer` builds a layer from a goose set and refuses any
file without a `-- +goose Down` section.

**Requirement.** What a test declares (`datatesting.Requirement`): the layer
names it needs (`Stack`, nil for all), how clean the store must be
(`Tolerant` accepts other tests' leftovers; `Pristine` needs exactly the
migrated state and holds the store exclusively), and whether it wants a
`Dedicated` store of its own.

**Planner.** Turns the store's recorded state and the requirement into the
cheapest exact plan: keep the matching prefix of layers; if pristine is
needed and the store is dirty, `Reset` the bottom layer when it can and
otherwise `Down` everything; `Down` what the test does not want, top first;
`Up` what it lacks. Per-test work — the plan, and the drop of a dedicated
store — runs as the test ends via `t.Cleanup`; only shared stores wait for
`Registry.Cleanup()` after `m.Run()`. A failed step marks the store broken,
fails that test, and has the next test drop and recreate it.

```go
Registry.GetFixtures(
    datatesting.Require(datatesting.Requirement{Stack: []string{"schema"}, Tolerance: datatesting.Pristine}),
    datatesting.WithFilter(...),
)
```

The bring-up module (`pkg/bringup/gorm`) closes its pool when the fx app
stops. Do not set `MaxIdleConns: 0` to compensate for pools that were never
closed — every statement then pays a full connection handshake, which on
YugabyteDB is two orders of magnitude more than the statement itself.
