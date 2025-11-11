package data_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/labels"
)

type LabelDummy struct {
	Labels model.Labels
}

func (l *LabelDummy) GetLabels() model.Labels {
	return l.Labels
}

func TestFilterByLabels(t *testing.T) {

	type s struct {
		arrange func() (labels.Selector, []*LabelDummy)
		assert  func(got []*LabelDummy, err error)
	}

	lbls, err := labels.Parse("a=1")

	if err != nil {
		panic(err)
	}

	cases := map[string]s{
		"nil": {
			arrange: func() (labels.Selector, []*LabelDummy) {
				return nil, nil
			},
			assert: func(got []*LabelDummy, err error) {
				require.NoError(t, err)
				assert.Nil(t, got)
			},
		},
		"match 1": {
			arrange: func() (labels.Selector, []*LabelDummy) {
				return lbls, []*LabelDummy{
					{
						Labels: map[string]string{
							"a": "1",
						},
					},
					{
						Labels: map[string]string{
							"a": "2",
						},
					},
				}
			},
			assert: func(got []*LabelDummy, err error) {
				require.NoError(t, err)
				assert.Equal(t, []*LabelDummy{
					{
						Labels: map[string]string{
							"a": "1",
						},
					},
				}, got)
			},
		},
	}

	for k, v := range cases {
		t.Run(k, func(_ *testing.T) {
			v.assert(repository.FilterByLabels(v.arrange()))
		})
	}
}

func TestFilterByLabelsNoLabels(t *testing.T) {

	lbls, err := labels.Parse("a=1")

	if err != nil {
		panic(err)
	}

	got, err := repository.FilterByLabels[*Dummy](lbls, []*Dummy{
		{},
		{},
	})

	assert.Nil(t, got)
	assert.EqualError(t, err, "type does not have labels to select")

}
