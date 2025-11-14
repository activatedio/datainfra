package gorm_test

import (
	"fmt"
	"testing"
	"time"

	datagorm "github.com/activatedio/datainfra/pkg/data/gorm"
	"github.com/activatedio/datainfra/pkg/setup"
	"github.com/activatedio/datainfra/pkg/setup/gorm"
	"github.com/stretchr/testify/require"
)

func TestSetup_Success(t *testing.T) {

	r := require.New(t)

	type s struct {
		arrange func() gorm.SetupParams
	}

	cases := map[string]s{
		"default": {
			arrange: func() gorm.SetupParams {

				now := time.Now().UnixMilli()
				name := fmt.Sprintf("test_%d", now)

				return gorm.SetupParams{
					OwnerConfig: &gorm.OwnerGormConfig{
						GormConfig: datagorm.GormConfig{
							Dialect:  "psql",
							Host:     "127.0.0.1",
							Port:     5432,
							Username: "postgres",
							Password: "supersecret",
							Name:     "postgres",
						},
					},
					AppConfig: &datagorm.GormConfig{
						Dialect:  "psql",
						Host:     "127.0.0.1",
						Port:     5432,
						Username: name,
						Password: name,
						Name:     name,
					},
				}
			},
		},
	}

	for k, v := range cases {
		t.Run(k, func(_ *testing.T) {

			unit := gorm.NewSetup(v.arrange())

			err := unit.Teardown()

			r.NoError(err)

			err = unit.Setup(setup.Params{FailOnExisting: true})

			r.NoError(err)

			err = unit.Setup(setup.Params{FailOnExisting: false})

			r.NoError(err)

			err = unit.Setup(setup.Params{FailOnExisting: true})

			r.ErrorAs(err, &setup.ResourceExistsError{})

			err = unit.Teardown()

			r.NoError(err)

			err = unit.Setup(setup.Params{FailOnExisting: true})

			r.NoError(err)

			err = unit.Teardown()

			r.NoError(err)

		})
	}

}
