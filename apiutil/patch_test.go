package apiutil

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func assertNoChange[T any](t *testing.T, old, new *T, exclude ...string) {
	t.Helper()
	skip := make(map[string]struct{}, len(exclude))
	for _, f := range exclude {
		skip[f] = struct{}{}
	}

	ov := reflect.ValueOf(old).Elem()
	nv := reflect.ValueOf(new).Elem()
	rt := ov.Type()

	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		jsonKey := jsonFieldName(field)
		if jsonKey == "" || jsonKey == "-" {
			continue
		}
		if _, ok := skip[jsonKey]; ok {
			continue
		}
		assert.Equalf(t, ov.Field(i).Interface(), nv.Field(i).Interface(), "field %q should not have changed", jsonKey)
	}
}

func TestPatch(t *testing.T) {
	type User struct {
		ID          string            `json:"id"`
		Username    string            `json:"username"`
		Email       string            `json:"email"`
		DisplayName string            `json:"display_name"`
		Age         int               `json:"age"`
		IsAdmin     bool              `json:"is_admin"`
		Groups      []string          `json:"groups"`
		Tags        map[string]string `json:"tags"`
		Score       *float64          `json:"score"`
	}

	score := 9.5
	baseline := func() User {
		return User{
			ID:          "u1",
			Username:    "rusty",
			Email:       "rusty@ocean.com",
			DisplayName: "Rusty Ryan",
			Age:         35,
			IsAdmin:     false,
			Groups:      []string{"crew", "planning"},
			Tags:        map[string]string{"role": "strategist"},
			Score:       &score,
		}
	}

	t.Run("Email", func(t *testing.T) {
		t.Run("set", func(t *testing.T) {
			u := baseline()
			p, err := NewPatchFromBytes[User]([]byte(`{"email":"new@ocean.com"}`))
			require.NoError(t, err)
			result, err := p.Apply(&u)
			require.NoError(t, err)
			assert.Equal(t, "new@ocean.com", result.Patched.Email)
			assertNoChange(t, result.Original, result.Patched, "email")
		})
		t.Run("reset", func(t *testing.T) {
			u := baseline()
			p, err := NewPatchFromBytes[User]([]byte(`{"email":null}`))
			require.NoError(t, err)
			result, err := p.Apply(&u)
			require.NoError(t, err)
			assert.Equal(t, "", result.Patched.Email)
			assertNoChange(t, result.Original, result.Patched, "email")
		})
		t.Run("ignore", func(t *testing.T) {
			u := baseline()
			p, err := NewPatchFromBytes[User]([]byte(`{}`))
			require.NoError(t, err)
			result, err := p.Apply(&u)
			require.NoError(t, err)
			assertNoChange(t, result.Original, result.Patched)
		})
	})

	t.Run("DisplayName", func(t *testing.T) {
		t.Run("set", func(t *testing.T) {
			u := baseline()
			p, err := NewPatchFromBytes[User]([]byte(`{"display_name":"Danny Ocean"}`))
			require.NoError(t, err)
			result, err := p.Apply(&u)
			require.NoError(t, err)
			assert.Equal(t, "Danny Ocean", result.Patched.DisplayName)
			assertNoChange(t, result.Original, result.Patched, "display_name")
		})
		t.Run("reset", func(t *testing.T) {
			u := baseline()
			p, err := NewPatchFromBytes[User]([]byte(`{"display_name":null}`))
			require.NoError(t, err)
			result, err := p.Apply(&u)
			require.NoError(t, err)
			assert.Equal(t, "", result.Patched.DisplayName)
			assertNoChange(t, result.Original, result.Patched, "display_name")
		})
		t.Run("ignore", func(t *testing.T) {
			u := baseline()
			p, err := NewPatchFromBytes[User]([]byte(`{}`))
			require.NoError(t, err)
			result, err := p.Apply(&u)
			require.NoError(t, err)
			assertNoChange(t, result.Original, result.Patched)
		})
	})

	t.Run("Age", func(t *testing.T) {
		t.Run("set", func(t *testing.T) {
			u := baseline()
			p, err := NewPatchFromBytes[User]([]byte(`{"age":40}`))
			require.NoError(t, err)
			result, err := p.Apply(&u)
			require.NoError(t, err)
			assert.Equal(t, 40, result.Patched.Age)
			assertNoChange(t, result.Original, result.Patched, "age")
		})
		t.Run("reset", func(t *testing.T) {
			u := baseline()
			p, err := NewPatchFromBytes[User]([]byte(`{"age":null}`))
			require.NoError(t, err)
			result, err := p.Apply(&u)
			require.NoError(t, err)
			assert.Equal(t, 0, result.Patched.Age)
			assertNoChange(t, result.Original, result.Patched, "age")
		})
		t.Run("ignore", func(t *testing.T) {
			u := baseline()
			p, err := NewPatchFromBytes[User]([]byte(`{}`))
			require.NoError(t, err)
			result, err := p.Apply(&u)
			require.NoError(t, err)
			assertNoChange(t, result.Original, result.Patched)
		})
	})

	t.Run("IsAdmin", func(t *testing.T) {
		t.Run("set", func(t *testing.T) {
			u := baseline()
			p, err := NewPatchFromBytes[User]([]byte(`{"is_admin":true}`))
			require.NoError(t, err)
			result, err := p.Apply(&u)
			require.NoError(t, err)
			assert.True(t, result.Patched.IsAdmin)
			assertNoChange(t, result.Original, result.Patched, "is_admin")
		})
		t.Run("reset", func(t *testing.T) {
			u := baseline()
			u.IsAdmin = true
			p, err := NewPatchFromBytes[User]([]byte(`{"is_admin":null}`))
			require.NoError(t, err)
			result, err := p.Apply(&u)
			require.NoError(t, err)
			assert.False(t, result.Patched.IsAdmin)
			assertNoChange(t, result.Original, result.Patched, "is_admin")
		})
		t.Run("ignore", func(t *testing.T) {
			u := baseline()
			p, err := NewPatchFromBytes[User]([]byte(`{}`))
			require.NoError(t, err)
			result, err := p.Apply(&u)
			require.NoError(t, err)
			assertNoChange(t, result.Original, result.Patched)
		})
	})

	t.Run("Groups", func(t *testing.T) {
		t.Run("set", func(t *testing.T) {
			u := baseline()
			p, err := NewPatchFromBytes[User]([]byte(`{"groups":["solo","heist"]}`))
			require.NoError(t, err)
			result, err := p.Apply(&u)
			require.NoError(t, err)
			assert.Equal(t, []string{"solo", "heist"}, result.Patched.Groups)
			assertNoChange(t, result.Original, result.Patched, "groups")
		})
		t.Run("reset", func(t *testing.T) {
			u := baseline()
			p, err := NewPatchFromBytes[User]([]byte(`{"groups":null}`))
			require.NoError(t, err)
			result, err := p.Apply(&u)
			require.NoError(t, err)
			assert.Nil(t, result.Patched.Groups)
			assertNoChange(t, result.Original, result.Patched, "groups")
		})
		t.Run("ignore", func(t *testing.T) {
			u := baseline()
			p, err := NewPatchFromBytes[User]([]byte(`{}`))
			require.NoError(t, err)
			result, err := p.Apply(&u)
			require.NoError(t, err)
			assertNoChange(t, result.Original, result.Patched)
		})
	})

	t.Run("Tags", func(t *testing.T) {
		t.Run("set", func(t *testing.T) {
			u := baseline()
			p, err := NewPatchFromBytes[User]([]byte(`{"tags":{"env":"prod"}}`))
			require.NoError(t, err)
			result, err := p.Apply(&u)
			require.NoError(t, err)
			assert.Equal(t, map[string]string{"env": "prod"}, result.Patched.Tags)
			assertNoChange(t, result.Original, result.Patched, "tags")
		})
		t.Run("reset", func(t *testing.T) {
			u := baseline()
			p, err := NewPatchFromBytes[User]([]byte(`{"tags":null}`))
			require.NoError(t, err)
			result, err := p.Apply(&u)
			require.NoError(t, err)
			assert.Nil(t, result.Patched.Tags)
			assertNoChange(t, result.Original, result.Patched, "tags")
		})
		t.Run("ignore", func(t *testing.T) {
			u := baseline()
			p, err := NewPatchFromBytes[User]([]byte(`{}`))
			require.NoError(t, err)
			result, err := p.Apply(&u)
			require.NoError(t, err)
			assertNoChange(t, result.Original, result.Patched)
		})
	})

	t.Run("Score", func(t *testing.T) {
		t.Run("set", func(t *testing.T) {
			u := baseline()
			p, err := NewPatchFromBytes[User]([]byte(`{"score":7.0}`))
			require.NoError(t, err)
			result, err := p.Apply(&u)
			require.NoError(t, err)
			require.NotNil(t, result.Patched.Score)
			assert.Equal(t, 7.0, *result.Patched.Score)
			assertNoChange(t, result.Original, result.Patched, "score")
		})
		t.Run("reset", func(t *testing.T) {
			u := baseline()
			p, err := NewPatchFromBytes[User]([]byte(`{"score":null}`))
			require.NoError(t, err)
			result, err := p.Apply(&u)
			require.NoError(t, err)
			assert.Nil(t, result.Patched.Score)
			assertNoChange(t, result.Original, result.Patched, "score")
		})
		t.Run("ignore", func(t *testing.T) {
			u := baseline()
			p, err := NewPatchFromBytes[User]([]byte(`{}`))
			require.NoError(t, err)
			result, err := p.Apply(&u)
			require.NoError(t, err)
			assertNoChange(t, result.Original, result.Patched)
		})
	})

	t.Run("OriginalNotMutated", func(t *testing.T) {
		u := baseline()
		p, err := NewPatchFromBytes[User]([]byte(`{"email":"new@ocean.com","age":40}`))
		require.NoError(t, err)
		_, err = p.Apply(&u)
		require.NoError(t, err)
		assert.Equal(t, baseline(), u)
	})

	t.Run("PointerIdentity", func(t *testing.T) {
		u := baseline()
		p, err := NewPatchFromBytes[User]([]byte(`{"email":"new@ocean.com"}`))
		require.NoError(t, err)
		result, err := p.Apply(&u)
		require.NoError(t, err)
		assert.Same(t, &u, result.Original)
		assert.NotSame(t, &u, result.Patched)
	})

	t.Run("IncludeFields", func(t *testing.T) {
		t.Run("permitted", func(t *testing.T) {
			u := baseline()
			p, err := NewPatchFromBytes[User]([]byte(`{"email":"new@ocean.com"}`))
			require.NoError(t, err)
			result, err := p.Apply(&u, IncludeFields("email", "display_name"))
			require.NoError(t, err)
			assert.Equal(t, "new@ocean.com", result.Patched.Email)
			assertNoChange(t, result.Original, result.Patched, "email")
		})
		t.Run("forbidden", func(t *testing.T) {
			u := baseline()
			p, err := NewPatchFromBytes[User]([]byte(`{"is_admin":true}`))
			require.NoError(t, err)
			_, err = p.Apply(&u, IncludeFields("email", "display_name"))
			require.Error(t, err)
		})
	})

	t.Run("ExcludeFields", func(t *testing.T) {
		t.Run("blocked", func(t *testing.T) {
			u := baseline()
			p, err := NewPatchFromBytes[User]([]byte(`{"is_admin":true}`))
			require.NoError(t, err)
			_, err = p.Apply(&u, ExcludeFields("is_admin"))
			require.Error(t, err)
		})
		t.Run("other fields work", func(t *testing.T) {
			u := baseline()
			p, err := NewPatchFromBytes[User]([]byte(`{"email":"new@ocean.com"}`))
			require.NoError(t, err)
			result, err := p.Apply(&u, ExcludeFields("is_admin"))
			require.NoError(t, err)
			assert.Equal(t, "new@ocean.com", result.Patched.Email)
			assertNoChange(t, result.Original, result.Patched, "email")
		})
	})

	t.Run("Diff", func(t *testing.T) {
		t.Run("changed fields", func(t *testing.T) {
			u := baseline()
			p, err := NewPatchFromBytes[User]([]byte(`{"email":"new@ocean.com","age":40}`))
			require.NoError(t, err)
			result, err := p.Apply(&u)
			require.NoError(t, err)
			diff := result.Diff()
			assert.Len(t, diff, 2)
			assert.Equal(t, "rusty@ocean.com", diff["email"].Old)
			assert.Equal(t, "new@ocean.com", diff["email"].New)
			assert.Equal(t, 35, diff["age"].Old)
			assert.Equal(t, 40, diff["age"].New)
		})
		t.Run("same value not included", func(t *testing.T) {
			u := baseline()
			p, err := NewPatchFromBytes[User]([]byte(`{"email":"rusty@ocean.com"}`))
			require.NoError(t, err)
			result, err := p.Apply(&u)
			require.NoError(t, err)
			assert.Empty(t, result.Diff())
		})
		t.Run("null reset", func(t *testing.T) {
			u := baseline()
			p, err := NewPatchFromBytes[User]([]byte(`{"display_name":null}`))
			require.NoError(t, err)
			result, err := p.Apply(&u)
			require.NoError(t, err)
			diff := result.Diff()
			assert.Len(t, diff, 1)
			assert.Equal(t, "Rusty Ryan", diff["display_name"].Old)
			assert.Equal(t, "", diff["display_name"].New)
		})
		t.Run("empty patch", func(t *testing.T) {
			u := baseline()
			p, err := NewPatchFromBytes[User]([]byte(`{}`))
			require.NoError(t, err)
			result, err := p.Apply(&u)
			require.NoError(t, err)
			assert.Empty(t, result.Diff())
		})
	})
}
