package handler

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePointsFromQuery(t *testing.T) {
	t.Run("Laravel indexed points[0]..points[3]", func(t *testing.T) {
		q := url.Values{}
		q.Set("points[0]", "10,20")
		q.Set("points[1]", "30,20")
		q.Set("points[2]", "30,40")
		q.Set("points[3]", "10,40")

		points, ok := parsePointsFromQuery(q)
		require.True(t, ok)
		assert.Equal(t, []string{"10,20", "30,20", "30,40", "10,40"}, points)
	})

	t.Run("points[] repeated values", func(t *testing.T) {
		q := url.Values{}
		q.Add("points[]", "10,20")
		q.Add("points[]", "30,20")
		q.Add("points[]", "30,40")
		q.Add("points[]", "10,40")

		points, ok := parsePointsFromQuery(q)
		require.True(t, ok)
		assert.Len(t, points, 4)
	})

	t.Run("JSON array in points", func(t *testing.T) {
		q := url.Values{}
		q.Set("points", `["10,20","30,20","30,40","10,40"]`)

		points, ok := parsePointsFromQuery(q)
		require.True(t, ok)
		assert.Equal(t, []string{"10,20", "30,20", "30,40", "10,40"}, points)
	})

	t.Run("missing points", func(t *testing.T) {
		_, ok := parsePointsFromQuery(url.Values{})
		assert.False(t, ok)
	})

	t.Run("does not split single points by comma", func(t *testing.T) {
		q := url.Values{}
		q.Set("points", "10,20,30,20,30,40,10,40")

		_, ok := parsePointsFromQuery(q)
		assert.False(t, ok)
	})
}
