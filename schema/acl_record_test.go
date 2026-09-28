package schema_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func upsertAcl(t *testing.T, tenantID, aclSlug, valueName string) {
	t.Helper()

	record := &schema.AclRecord{
		Slug:  aclSlug,
		Value: datatypes.NewJSONType(schema.Acl{ID: aclSlug, Name: valueName}),
	}
	require.NoError(t, record.Upsert(tenantCtx(tenantID)))
}

func TestAclRecord_Upsert(t *testing.T) {
	ctx := setupSchemaTest(t)
	upsertAcl(t, "tenant-1", "net-1.all-nodes", "all nodes")

	record := &schema.AclRecord{Slug: "net-1.all-nodes"}
	require.NoError(t, record.Get(ctx))
	_, err := uuid.Parse(record.ID)
	assert.NoError(t, err, "upsert generates a uuid id")
	assert.Equal(t, "tenant-1", record.TenantID)
	assert.Equal(t, "net-1.all-nodes", record.Slug, "the slug is stored without a tenant prefix")
	assert.Equal(t, "all nodes", record.Value.Data().Name)

	t.Run("UpdatesExisting", func(t *testing.T) {
		upsertAcl(t, "tenant-1", "net-1.all-nodes", "renamed")

		updated := &schema.AclRecord{Slug: "net-1.all-nodes"}
		require.NoError(t, updated.Get(ctx))
		assert.Equal(t, record.ID, updated.ID, "the id is kept")
		assert.Equal(t, "renamed", updated.Value.Data().Name)

		count, err := (&schema.AclRecord{}).Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("SameSlugInAnotherTenant", func(t *testing.T) {
		upsertAcl(t, "tenant-2", "net-1.all-nodes", "other tenant")

		other := &schema.AclRecord{Slug: "net-1.all-nodes"}
		require.NoError(t, other.Get(tenantCtx("tenant-2")))
		assert.NotEqual(t, record.ID, other.ID)
		assert.Equal(t, "other tenant", other.Value.Data().Name)

		mine := &schema.AclRecord{Slug: "net-1.all-nodes"}
		require.NoError(t, mine.Get(ctx))
		assert.Equal(t, "renamed", mine.Value.Data().Name)
	})

	t.Run("UniqueTenantSlug", func(t *testing.T) {
		duplicate := &schema.AclRecord{ID: uuid.NewString(), TenantID: "tenant-1", Slug: "net-1.all-nodes"}
		assert.Error(t, db.FromContext(ctx).Create(duplicate).Error)
	})
}

func TestAclRecord_TenantScoped(t *testing.T) {
	ctx := setupSchemaTest(t)
	otherCtx := tenantCtx("tenant-2")
	upsertAcl(t, "tenant-1", "acl-1", "acl 1")
	upsertAcl(t, "tenant-1", "acl-2", "acl 2")
	upsertAcl(t, "tenant-2", "acl-1", "acl 1")

	records, err := (&schema.AclRecord{}).List(ctx)
	require.NoError(t, err)
	require.Len(t, records, 2)
	for _, record := range records {
		assert.Equal(t, "tenant-1", record.TenantID)
	}

	assert.ErrorIs(t, (&schema.AclRecord{Slug: "acl-2"}).Get(otherCtx), gorm.ErrRecordNotFound)

	require.NoError(t, (&schema.AclRecord{Slug: "acl-1"}).Delete(ctx))
	assert.ErrorIs(t, (&schema.AclRecord{Slug: "acl-1"}).Get(ctx), gorm.ErrRecordNotFound)
	assert.NoError(t, (&schema.AclRecord{Slug: "acl-1"}).Get(otherCtx), "delete is tenant scoped")

	require.NoError(t, (&schema.AclRecord{}).DeleteAll(ctx))
	count, err := (&schema.AclRecord{}).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
	count, err = (&schema.AclRecord{}).Count(otherCtx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestAclRecord_ByID(t *testing.T) {
	ctx := setupSchemaTest(t)
	upsertAcl(t, "tenant-1", "acl-1", "acl 1")

	bySlug := &schema.AclRecord{Slug: "acl-1"}
	require.NoError(t, bySlug.Get(ctx))

	t.Run("Get", func(t *testing.T) {
		byID := &schema.AclRecord{ID: bySlug.ID}
		require.NoError(t, byID.Get(ctx))
		assert.Equal(t, "acl-1", byID.Slug)

		// the id wins over the slug.
		mismatched := &schema.AclRecord{ID: bySlug.ID, Slug: "acl-other"}
		require.NoError(t, mismatched.Get(ctx))
		assert.Equal(t, "acl-1", mismatched.Slug)

		assert.ErrorIs(t, (&schema.AclRecord{ID: bySlug.ID}).Get(tenantCtx("tenant-2")), gorm.ErrRecordNotFound)
	})

	t.Run("Upsert", func(t *testing.T) {
		renamed := &schema.AclRecord{
			ID:    bySlug.ID,
			Slug:  "acl-renamed",
			Value: datatypes.NewJSONType(schema.Acl{Name: "renamed"}),
		}
		require.NoError(t, renamed.Upsert(ctx))

		got := &schema.AclRecord{ID: bySlug.ID}
		require.NoError(t, got.Get(ctx))
		assert.Equal(t, "acl-renamed", got.Slug, "upserting by id changes the slug")
		assert.Equal(t, "renamed", got.Value.Data().Name)

		count, err := (&schema.AclRecord{}).Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("Delete", func(t *testing.T) {
		require.NoError(t, (&schema.AclRecord{ID: bySlug.ID}).Delete(tenantCtx("tenant-2")))
		assert.NoError(t, (&schema.AclRecord{ID: bySlug.ID}).Get(ctx), "delete is tenant scoped")

		require.NoError(t, (&schema.AclRecord{ID: bySlug.ID}).Delete(ctx))
		assert.ErrorIs(t, (&schema.AclRecord{ID: bySlug.ID}).Get(ctx), gorm.ErrRecordNotFound)
	})

	t.Run("MissingIdentifiers", func(t *testing.T) {
		assert.ErrorIs(t, (&schema.AclRecord{}).Get(ctx), schema.ErrAclIdentifiersNotProvided)
		assert.ErrorIs(t, (&schema.AclRecord{}).Delete(ctx), schema.ErrAclIdentifiersNotProvided)
	})
}
