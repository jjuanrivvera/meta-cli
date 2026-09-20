package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlexibleTypes(t *testing.T) {
	var id ID
	require.NoError(t, json.Unmarshal([]byte(`9007199254740993`), &id))
	assert.Equal(t, ID("9007199254740993"), id)
	require.NoError(t, json.Unmarshal([]byte(`"42"`), &id))
	assert.Equal(t, ID("42"), id)

	var integer Int
	require.NoError(t, json.Unmarshal([]byte(`"-12"`), &integer))
	assert.Equal(t, Int(-12), integer)
	assert.Error(t, json.Unmarshal([]byte(`1.5`), &integer))

	for input, expected := range map[string]bool{`true`: true, `"yes"`: true, `"0"`: false} {
		var boolean Bool
		require.NoError(t, json.Unmarshal([]byte(input), &boolean))
		assert.Equal(t, expected, bool(boolean))
	}
	var decimal Money
	require.NoError(t, json.Unmarshal([]byte(`"123.4500"`), &decimal))
	assert.Equal(t, Money("123.4500"), decimal)
	assert.Error(t, json.Unmarshal([]byte(`"NaN"`), &decimal))

	var stringsValue StringOrSlice
	require.NoError(t, json.Unmarshal([]byte(`"one"`), &stringsValue))
	assert.Equal(t, StringOrSlice{"one"}, stringsValue)
	require.NoError(t, json.Unmarshal([]byte(`["one","two"]`), &stringsValue))
	assert.Len(t, stringsValue, 2)

	var refs Refs
	require.NoError(t, json.Unmarshal([]byte(`{"id":"1","name":"Page"}`), &refs))
	assert.Len(t, refs, 1)
	assert.Error(t, json.Unmarshal([]byte(`true`), &refs))
}

func TestIDMarshalAndInvalidValues(t *testing.T) {
	data, err := json.Marshal(ID("123"))
	require.NoError(t, err)
	assert.JSONEq(t, `"123"`, string(data))
	var id ID
	assert.Error(t, json.Unmarshal([]byte(`true`), &id))
	var boolean Bool
	assert.Error(t, json.Unmarshal([]byte(`"maybe"`), &boolean))
	var values StringOrSlice
	assert.Error(t, json.Unmarshal([]byte(`12`), &values))
}

func FuzzID(f *testing.F) {
	for _, seed := range []string{`"1"`, `1`, `null`, `true`, `{}`} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		var value ID
		_ = json.Unmarshal([]byte(input), &value)
	})
}

func FuzzFlexibleInt(f *testing.F) {
	for _, seed := range []string{`"1"`, `-1`, `1.2`, `"NaN"`} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		var value Int
		_ = json.Unmarshal([]byte(input), &value)
	})
}
