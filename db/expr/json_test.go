package expr

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKeysAreBound(t *testing.T) {
	const key = `o'brien{x}@corp.com`
	tests := []struct {
		dialect  string
		remove   string
		hasKey   string
		notNull  string
		keyValue interface{}
	}{
		{
			dialect:  "sqlite",
			remove:   "json_remove(users, ?, ?)",
			hasKey:   "json_type(users, ?) IS NOT NULL",
			notNull:  "json_extract(users, ?) IS NOT NULL",
			keyValue: `$."o'brien{x}@corp.com"`,
		},
		{
			dialect:  "postgres",
			remove:   "users - ?::text - ?::text",
			hasKey:   "jsonb_exists(users, ?::text)",
			notNull:  "users->>?::text IS NOT NULL",
			keyValue: key,
		},
	}
	for _, tt := range tests {
		t.Run(tt.dialect, func(t *testing.T) {
			t.Setenv("DATABASE", tt.dialect)

			rm := Remove("users", key, key)
			assert.Equal(t, tt.remove, rm.SQL)
			assert.Equal(t, []interface{}{tt.keyValue, tt.keyValue}, rm.Vars)

			hk := WhereHasKey("users", key)
			assert.Equal(t, tt.hasKey, hk.SQL)
			assert.Equal(t, []interface{}{tt.keyValue}, hk.Vars)

			nn := WhereNotNull("users", key)
			assert.Equal(t, tt.notNull, nn.SQL)
			assert.Equal(t, []interface{}{tt.keyValue}, nn.Vars)

			w := Where("users", key, Gt, 1)
			assert.Equal(t, []interface{}{tt.keyValue, 1}, w.Vars)

			s := Set("users", key, "v")
			assert.Equal(t, []interface{}{tt.keyValue, "v"}, s.Vars)
			assert.NotContains(t, s.SQL, "brien")
		})
	}
}
