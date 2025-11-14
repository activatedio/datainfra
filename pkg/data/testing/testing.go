package testing

import (
	"context"
	"reflect"
	"testing"

	"github.com/activatedio/datainfra/pkg/data"
	"github.com/activatedio/datainfra/pkg/symbols"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"k8s.io/apimachinery/pkg/labels"
)

func RandomLabels() data.Labels {
	return map[string]string{
		// TODO - better to have another uuid provider
		"a1": uuid.New().String(),
		"a2": uuid.New().String(),
	}
}

func Run(t *testing.T, fixtures []AppFixture, toInvoke any, toProvide ...any) {

	for _, fix := range fixtures {

		res := fix.GetApp(t, toInvoke, toProvide...)

		t.Run(res.Name, func(t *testing.T) {
			res.App.RequireStart()

			res.App.RequireStop()
		})

	}
}

/*
func Run(t *testing.T, profiles []string, callback func(t *testing.T, ctx context.Context, profile string), opts ...fx.Option) {

	for _, profile := range profiles {
		t.Run(profile, func(t *testing.T) {

			var repositoryMode string
			var metadataRepositoryMode string
			var testSource func() symbols.Symbols

			switch profile {
			case ProfileGormPsql:
				repositoryMode = repository.ModeGorm
				metadataRepositoryMode = repository.ModeGorm
				testSource = testdata.SourceMain
			case ProfileGormSqlite:
				repositoryMode = repository.ModeGorm
				metadataRepositoryMode = repository.ModeGorm
				testSource = testdata.SourceMain
			case ProfileGocql:
				repositoryMode = repository.ModeGocql
				metadataRepositoryMode = repository.ModeGocql
				testSource = testdata.SourceMain
			case ProfileGormPsqlStaticMetadata:
				repositoryMode = repository.ModeGorm
				metadataRepositoryMode = repository.ModeStatic
				testSource = testdata.SourceMainStaticMetadata
			case ProfileGormSqliteStaticMetadata:
				repositoryMode = repository.ModeGorm
				metadataRepositoryMode = repository.ModeStatic
				testSource = testdata.SourceMainStaticMetadata
			case ProfileGocqlStaticMetadata:
				repositoryMode = repository.ModeGocql
				metadataRepositoryMode = repository.ModeStatic
				testSource = testdata.SourceMainStaticMetadata
			default:
				panic("unrecognized profile: " + profile)
			}

			done := NewDone()

			type InvokeParams struct {
				fx.In
				ContextBuilder data.ContextBuilder
				Migrator       migrate.Migrator `name:"repository_inner"`
			}

			addGormMigrations := func(mainFiles, testFiles embed.FS, in ...any) []any {
				return append(in, []any{
					fx.Annotate(
						func() *gorm.Migrations {
							return &gorm.Migrations{
								Sources: map[string]*gorm.MigrationSource{
									"main": {
										// Main difference is that this is true
										Drop: true,
										FS:   mainFiles,
										//FS:   gorm_migrations.Main,
										Path: "main",
									},
								},
								Symbols: testSource(),
							}
						}, fx.ResultTags(`name:"main"`)),
					fx.Annotate(
						func() *gorm.Migrations {
							return &gorm.Migrations{
								Sources: map[string]*gorm.MigrationSource{
									"test": {
										Drop: false,
										FS:   testFiles,
										//FS:   gorm_migrations.Test,
										Path: "test",
									},
								},
								Symbols: testSource(),
							}
						}, fx.ResultTags(`name:"extended"`)),
				}...)
			}

			addGocqlMigrations := func(mainFiles, testFiles embed.FS, in ...any) []any {
				return append(in, []any{

					fx.Annotate(
						func() *gocql.Migrations {
							return &gocql.Migrations{
								Sources: map[string]*gocql.MigrationSource{
									"main": {
										// Main difference is that this is true
										//FS: gocql_migrations_main.Files,
										FS: mainFiles,
									},
								},
								Symbols: map[string]any{
									"AppUser": "app",
								},
							}
						}, fx.ResultTags(`name:"main"`)),
					fx.Annotate(
						func() *gocql.Migrations {
							return &gocql.Migrations{
								Sources: map[string]*gocql.MigrationSource{
									"test": {
										//FS: gocql_migrations_test.Files,
										FS: testFiles,
									},
								},
								Symbols: testSource(),
							}
						}, fx.ResultTags(`name:"extended"`)),
				}...)
			}

			c := cs.New()

			c.AddSource(sources.FromValue(runtime.PrefixRepositoryCommon, &runtime.RepositoryConfig{
				PrimaryMode:  repositoryMode,
				MetadataMode: metadataRepositoryMode,
			}))

			rConfig := runtime.NewRepositoryConfig(c)

			app := fx.New(fx.Provide(testSource), outer.Index(), core_fx.RepositoryInnerIndex(rConfig, core_fx.NewIndexOptions(
				core_fx.ExcludeMigrations(),
				core_fx.ExcludeRepositoryConfig(),
			)), func() fx.Option {
				switch profile {
				case ProfileGormPsql:
					return fx.Module("test", fx.Provide(
						addGormMigrations(gorm_migrations.Main, gorm_migrations.Test,
							func() runtime.GormAppConfigResult {
								return runtime.GormAppConfigResult{
									Result: gormConfigPsql,
								}
							},
						)...,
					))
				case ProfileGormPsqlStaticMetadata:
					return fx.Module("test",
						fx.Provide(func() *static.Root {
							d, err := static.LoadData(bytes.NewReader(testdata.TestData))
							crypto.Check(err)
							return d
						}),
						fx.Provide(
							addGormMigrations(gorm_migrations_no_metadata.Main, gorm_migrations_no_metadata.Test,
								func() runtime.GormAppConfigResult {
									return runtime.GormAppConfigResult{
										Result: gormConfigPsqlLite,
									}
								},
							)...,
						))
				case ProfileGormSqlite:
					return fx.Module("test", fx.Provide(
						addGormMigrations(gorm_migrations.Main, gorm_migrations.Test,
							func() runtime.GormAppConfigResult {
								return runtime.GormAppConfigResult{
									Result: gormConfigSqlite,
								}
							},
						)...,
					))
				case ProfileGormSqliteStaticMetadata:
					return fx.Module("test",
						fx.Provide(func() *static.Root {
							d, err := static.LoadData(bytes.NewReader(testdata.TestData))
							crypto.Check(err)
							return d
						}),
						fx.Provide(
							addGormMigrations(gorm_migrations_no_metadata.Main, gorm_migrations_no_metadata.Test,
								func() runtime.GormAppConfigResult {
									return runtime.GormAppConfigResult{
										Result: gormConfigSqliteLite,
									}
								},
							)...,
						))
				case ProfileGocql:
					return fx.Module("test", fx.Provide(
						addGocqlMigrations(gocql_migrations_main.Files, gocql_migrations_test.Files,
							func() *runtime.IndexConfig {
								return &runtime.IndexConfig{
									Prefix: gocqlConfig.Keyspace,
								}
							},
							func() *runtime.ElasticsearchConfig {
								return &runtime.ElasticsearchConfig{
									ElasticSearchEndpoint: "http://127.0.0.1:9200",
								}
							},
							func() runtime.GocqlOwnerConfigResult {
								return runtime.GocqlOwnerConfigResult{
									Result: gocqlOwnerConfig,
								}
							},
							func() runtime.GocqlAppConfigResult {
								return runtime.GocqlAppConfigResult{
									Result: gocqlConfig,
								}
							},
							loopback.NewAuthwiseDataflowServiceClient,
						)...,
					),
					)
				case ProfileGocqlStaticMetadata:
					return fx.Module("test",
						fx.Provide(func() *static.Root {
							d, err := static.LoadData(bytes.NewReader(testdata.TestData))
							crypto.Check(err)
							return d
						}),
						fx.Provide(
							addGocqlMigrations(gocql_migrations_main_no_metadata.Files, gocql_migrations_test_no_metadata.Files,
								func() *runtime.IndexConfig {
									return &runtime.IndexConfig{
										Prefix: gocqlLiteConfig.Keyspace,
									}
								},
								func() *runtime.ElasticsearchConfig {
									return &runtime.ElasticsearchConfig{
										ElasticSearchEndpoint: "http://127.0.0.1:9200",
									}
								},
								func() runtime.GocqlOwnerConfigResult {
									return runtime.GocqlOwnerConfigResult{
										Result: gocqlOwnerConfig,
									}
								},
								func() runtime.GocqlAppConfigResult {
									return runtime.GocqlAppConfigResult{
										Result: gocqlLiteConfig,
									}
								},
								loopback.NewAuthwiseDataflowServiceClient,
							)...,
						),
					)
				default:
					panic("unrecognized profile: " + profile)
				}
			}(), fx.Module("fixture", opts...),
				fx.Invoke(func(params InvokeParams) {

					defer done.Done()

					cb := params.ContextBuilder
					mig := params.Migrator

					ctx := cb.Build(context.Background())

					MigrateSync.Lock()
					switch profile {
					case ProfileGormPsql:
						if !GormPsqlMigrated {
							db := gorm.GetDB(ctx)
							crypto.Check(mig.Migrate(gorm.WithDB(ctx, db)))
							GormPsqlMigrated = true
						}
					case ProfileGormSqlite:
						if !GormSqliteMigrated {
							db := gorm.GetDB(ctx)
							crypto.Check(mig.Migrate(gorm.WithDB(ctx, db)))
							GormSqliteMigrated = true
						}
					case ProfileGocql:
						if !GocqlMigrated {
							crypto.Check(mig.Migrate(ctx))
							GocqlMigrated = true
						}
					case ProfileGormPsqlStaticMetadata:
						if !GormLitePsqlMigrated {
							db := gorm.GetDB(ctx)
							crypto.Check(mig.Migrate(gorm.WithDB(ctx, db)))
							GormLitePsqlMigrated = true
						}
					case ProfileGormSqliteStaticMetadata:
						if !GormLiteSqliteMigrated {
							db := gorm.GetDB(ctx)
							crypto.Check(mig.Migrate(gorm.WithDB(ctx, db)))
							GormLiteSqliteMigrated = true
						}
					case ProfileGocqlStaticMetadata:
						if !GocqlLiteMigrated {
							crypto.Check(mig.Migrate(ctx))
							GocqlLiteMigrated = true
						}
					default:
						panic("unrecognized profile: " + profile)
					}
					MigrateSync.Unlock()

					switch profile {
					case ProfileGormPsql:
						ctx = gorm.WithDB(ctx, gorm.GetDB(ctx).Begin())
					case ProfileGormSqlite:
						ctx = gorm.WithDB(ctx, gorm.GetDB(ctx).Begin())
					case ProfileGocql:
					case ProfileGormPsqlStaticMetadata:
						ctx = gorm.WithDB(ctx, gorm.GetDB(ctx).Begin())
					case ProfileGormSqliteStaticMetadata:
						ctx = gorm.WithDB(ctx, gorm.GetDB(ctx).Begin())
					case ProfileGocqlStaticMetadata:
					default:
						panic("unrecognized profile: " + profile)
					}

					callback(t, ctx, profile)

					closeGorm := func() {
						db := gorm.GetDB(ctx)
						crypto.Check(db.Rollback().Error)
						sdb, err := db.DB()
						crypto.Check(err)
						crypto.Check(sdb.Close())
					}

					switch profile {
					case ProfileGormPsql:
						closeGorm()
					case ProfileGormSqlite:
						closeGorm()
					case ProfileGocql:
						gocql.GetSession(ctx).Close()
					case ProfileGormPsqlStaticMetadata:
						closeGorm()
					case ProfileGormSqliteStaticMetadata:
						closeGorm()
					case ProfileGocqlStaticMetadata:
						gocql.GetSession(ctx).Close()
					default:
						panic("unrecognized profile: " + profile)
					}
				}))

			startCtx, cancel p= context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if err := app.Start(startCtx); err != nil {
				log.Fatal().Err(err).Msg("failed to start app")
			}

			runCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()

			select {
			case <-runCtx.Done():
				panic("test did not run after 20 seconds")
			case <-done:
				log.Info().Msg("unit test run")
			}

			stopCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if err := app.Stop(stopCtx); err != nil {
				log.Fatal().Err(err).Msg("failed to stop app")
			}

		})

	}
}

*/

type Unit[T any] struct {
	unit    T
	symbols symbols.Symbols
}

func NewUnit[T any]() *Unit[T] {
	return &Unit[T]{}
}

func (u *Unit[T]) UnitAndSymbols() (T, symbols.Symbols) {
	return u.unit, u.symbols
}

func (u *Unit[T]) Options() fx.Option {
	return fx.Module("unit", fx.Populate(&u.unit, &u.symbols))
}

type ListAssertion[E any] struct {
	ExpectedCount   int
	AssertListEntry func(t *testing.T, e E)
}

type SelectAssertion struct {
	Expression    string
	ExpectedCount int
}

type CrudTestFixture[E any, K comparable] struct {
	NewEntity          func() E
	KeyExists          K
	KeyMissing         K
	ExtractKey         func(e E) K
	SelectAssertions   []SelectAssertion
	ListAssertion      *ListAssertion[E]
	AssertDetailEntry  func(t *testing.T, e E)
	ModifyBeforeCreate func(e E)
	AssertAfterCreate  func(t *testing.T, e E)
	ModifyBeforeUpdate func(e E)
	AssertAfterUpdate  func(t *testing.T, e E)
}

func DoTestCrudRepository[E any, K comparable](t *testing.T,
	ctx context.Context, unit data.CrudTemplate[E, K], fixture *CrudTestFixture[E, K]) {

	for _, sa := range fixture.SelectAssertions {

		l, err := labels.Parse(sa.Expression)

		if err != nil {
			panic(err)
		}

		list, err := unit.ListAll(ctx, data.ListParams{
			Selector: l,
		})

		require.NoError(t, err)
		assert.Len(t, list.List, sa.ExpectedCount, sa.Expression)
	}

	if fixture.ListAssertion != nil {

		la := fixture.ListAssertion

		list, err := unit.ListAll(ctx, data.ListParams{})

		require.NoError(t, err)
		assert.Len(t, list.List, la.ExpectedCount)

		assert.NotNil(t, list.List)
		for _, v := range list.List {
			la.AssertListEntry(t, v)
		}
	}

	got, err := unit.FindByKey(ctx, fixture.KeyMissing)

	require.NoError(t, err)
	assert.Nil(t, got)

	got, err = unit.FindByKey(ctx, fixture.KeyExists)

	require.NoError(t, err)
	assert.NotNil(t, got)

	fixture.AssertDetailEntry(t, got)

	// Create with bad labels
	got = fixture.NewEntity()
	if HasLabels(got) {
		fixture.ModifyBeforeCreate(got)
		SetBadLabels(got)
		err = unit.Create(ctx, got)
		assert.Contains(t, err.Error(), "name part must consist")
	}
	// Create
	got = fixture.NewEntity()
	fixture.ModifyBeforeCreate(got)
	err = unit.Create(ctx, got)
	require.NoError(t, err)

	fixture.AssertAfterCreate(t, got)

	err = unit.Create(ctx, got)
	assert.True(t, errors.Is(err, data.EntityAlreadyExists{}))

	key := fixture.ExtractKey(got)

	got2, err := unit.FindByKey(ctx, key)
	require.NoError(t, err)

	fixture.AssertAfterCreate(t, got2)

	if fixture.AssertAfterUpdate != nil && fixture.ModifyBeforeUpdate != nil {

		fixture.ModifyBeforeUpdate(got)

		err = unit.Update(ctx, got)

		require.NoError(t, err)

		fixture.AssertAfterUpdate(t, got)

		if HasLabels(got) {
			SetBadLabels(got)
			err = unit.Update(ctx, got)
			assert.Contains(t, err.Error(), "name part must consist")
		}

		got2, err = unit.FindByKey(ctx, fixture.ExtractKey(got))
		require.NoError(t, err)
		fixture.AssertAfterUpdate(t, got2)

	}

	err = unit.Delete(ctx, key)
	require.NoError(t, err)

	got3, err := unit.FindByKey(ctx, key)

	require.NoError(t, err)
	assert.Nil(t, got3)

}

func SetBadLabels(got any) {

	f := reflect.ValueOf(got).Elem().FieldByName("Labels")
	f.Set(reflect.ValueOf(map[string]string{
		" b a d k e y": "__--**&&bdValue",
	}))
}

func HasLabels(got any) bool {
	_, ok := reflect.TypeOf(got).Elem().FieldByName("Labels")
	return ok
}

type FilterKeysTestFixture[K comparable] struct {
	UnitFactory    func() data.FilterKeysTemplate[K]
	ArrangeContext func(context.Context) context.Context
	KeyExists      K
	KeyMissing     K
}

/*
func DoTestFilterKeysRepository[K comparable, T data.FilterKeysTemplate[K]](t *testing.T, profiles []string, fixtureFactory func(symbols symbols.Symbols) *FilterKeysTestFixture[K]) {

	harness := NewUnit[T]()

	Run(t, profiles, func(t *testing.T, ctx context.Context, _ string) {

		unit, symbols := harness.UnitAndSymbols()

		fixture := fixtureFactory(symbols)

		ctx = fixture.ArrangeContext(ctx)

		got, err := unit.FilterKeys(ctx, []K{fixture.KeyExists, fixture.KeyMissing})

		require.NoError(t, err)
		assert.Equal(t, []K{fixture.KeyExists}, got)
	}, harness.Options())

}

*/
