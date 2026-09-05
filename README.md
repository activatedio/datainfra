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

`pkg/data/testing` gives every test suite the same four rings around a
database — **Setup → MigrateUp → [test] → MigrateDown → Teardown** — and three
ways to place a test inside them:

| Mode | Setup | MigrateUp | MigrateDown | Teardown |
|---|---|---|---|---|
| `ModeReuse` | shared database, once | base + delta, once | never | suite end (`Registry.Cleanup`) |
| `ModeReuseWithMigrate` | shared database, once | base once; **delta per test** | **delta per test**, via `t.Cleanup` | suite end |
| `ModeFresh` | this test's own database | base + delta | skipped — the drop is the undo | this test, via `t.Cleanup` |

Two tiers of migration feed the rings:

* **base** — the graph's untagged `migrate.Migrator` (`gormmigrate.NewMigrator`
  over the untagged `[]MigratorData`): schema, plus any data the database
  carries for life. Applied once per database, never reverted.
* **delta** — the graph's `migrate.Reversible` tagged `name:"delta"`: what one
  test needs on top and must remove afterwards. Build it from goose sets with
  `gormmigrate.NewDeltaMigrator` (every file must carry a `-- +goose Down`
  section, checked at construction), or compose one-directional loaders with
  `migrate.Sequence(migrate.UpOnly(loader), gormmigrate.NewTableReset(cfg))`,
  whose Down deletes every data table with DML — no DDL, which is what keeps
  it cheap on YugabyteDB.

Per-test rings run as soon as the test function ends, including after a
`require` failure, so a suite tears down as it goes; only shared databases
wait for `Registry.Cleanup()` after `m.Run()`.

```go
Registry.GetFixtures(datatesting.WithMode(datatesting.ModeReuseWithMigrate), datatesting.WithFilter(...))
```

The bring-up module (`pkg/bringup/gorm`) closes its pool when the fx app
stops. Do not set `MaxIdleConns: 0` to compensate for pools that were never
closed — every statement then pays a full connection handshake, which on
YugabyteDB is two orders of magnitude more than the statement itself.
