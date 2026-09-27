package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"gorm.io/gorm"
)

type extClientTestEnv struct {
	ctx     context.Context
	network *schema.Network
	gateway *schema.Node
}

func setupExtClientTest(t *testing.T, networkName string) *extClientTestEnv {
	t.Helper()
	t.Setenv("MESSAGEQUEUE_BACKEND", "off")

	ctx := scope.WithContext(db.WithContext(context.Background()), scope.TenantScope, defaultTenantID)

	network := &schema.Network{
		TenantID:     defaultTenantID,
		Name:         networkName,
		AddressRange: "10.20.0.0/24",
	}
	require.NoError(t, network.Create(ctx))

	host := &schema.Host{
		ID:       uuid.New(),
		TenantID: defaultTenantID,
		Name:     networkName + "-gw",
	}
	require.NoError(t, host.Create(ctx))

	gateway := &schema.Node{
		ID:        uuid.NewString(),
		TenantID:  defaultTenantID,
		HostID:    host.ID.String(),
		NetworkID: network.ID,
		Address:   "10.20.0.1/24",
		IsGateway: true,
	}
	require.NoError(t, gateway.Create(ctx))

	return &extClientTestEnv{ctx: ctx, network: network, gateway: gateway}
}

func (env *extClientTestEnv) createExtClient(t *testing.T, name string) models.ExtClient {
	t.Helper()

	key, err := wgtypes.GeneratePrivateKey()
	require.NoError(t, err)

	extclient := models.ExtClient{
		ClientID:         name,
		Network:          env.network.Name,
		Address:          "10.20.0.10",
		PublicKey:        key.PublicKey().String(),
		PrivateKey:       key.String(),
		IngressGatewayID: env.gateway.ID,
		Enabled:          true,
	}
	require.NoError(t, logic.SaveExtClient(env.ctx, &extclient))
	return extclient
}

func (env *extClientTestEnv) do(handler http.HandlerFunc, method string, vars map[string]string, body any) *httptest.ResponseRecorder {
	var reqBody bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&reqBody).Encode(body)
	}
	req := httptest.NewRequest(method, "/", &reqBody).WithContext(env.ctx)
	req.Header.Set("ismaster", "yes")
	req = mux.SetURLVars(req, vars)
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

func TestGetExtClientByNameOrID(t *testing.T) {
	env := setupExtClientTest(t, "ec-get-net")
	extclient := env.createExtClient(t, "ec-get-client")

	for _, clientid := range []string{extclient.ClientID, extclient.ID} {
		rec := env.do(getExtClient, http.MethodGet, map[string]string{"network": env.network.Name, "clientid": clientid}, nil)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		var got models.ExtClient
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&got))
		assert.Equal(t, extclient.ID, got.ID)
		assert.Equal(t, extclient.ClientID, got.ClientID)
	}

	t.Run("IDInAnotherNetwork", func(t *testing.T) {
		other := setupExtClientTest(t, "ec-get-net-2")
		rec := env.do(getExtClient, http.MethodGet, map[string]string{"network": other.network.Name, "clientid": extclient.ID}, nil)
		assert.NotEqual(t, http.StatusOK, rec.Code)
	})
}

func TestUpdateExtClient_Rename(t *testing.T) {
	env := setupExtClientTest(t, "ec-update-net")
	extclient := env.createExtClient(t, "ec-update-client")

	policy := models.Acl{
		ID:        uuid.NewString(),
		NetworkID: schema.NetworkID(env.network.Name),
		RuleType:  models.DevicePolicy,
		Src:       []models.AclPolicyTag{{ID: models.NodeID, Value: extclient.ClientID}},
		Dst:       []models.AclPolicyTag{{ID: models.NodeID, Value: "ec-other-client"}},
	}
	require.NoError(t, logic.UpsertAcl(env.ctx, policy))

	update := models.CustomExtClient{
		ClientID:  "ec-renamed-client",
		PublicKey: extclient.PublicKey,
		Enabled:   true,
	}
	rec := env.do(updateExtClient, http.MethodPut, map[string]string{"network": env.network.Name, "clientid": extclient.ID}, update)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	renamed, err := logic.GetExtClient(env.ctx, "ec-renamed-client", env.network.Name)
	require.NoError(t, err)
	assert.Equal(t, extclient.ID, renamed.ID, "rename keeps the id")

	_, err = logic.GetExtClient(env.ctx, extclient.ClientID, env.network.Name)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)

	got, err := logic.GetAcl(env.ctx, policy.ID)
	require.NoError(t, err)
	assert.Equal(t, "ec-renamed-client", got.Src[0].Value, "policies follow the rename")
}

func TestDeleteExtClientByID(t *testing.T) {
	env := setupExtClientTest(t, "ec-delete-net")
	extclient := env.createExtClient(t, "ec-delete-client")

	rec := env.do(deleteExtClient, http.MethodDelete, map[string]string{"network": env.network.Name, "clientid": extclient.ID}, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	_, err := logic.GetExtClient(env.ctx, extclient.ID, env.network.Name)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestGetNetworkExtClients_IncludesViolations(t *testing.T) {
	env := setupExtClientTest(t, "ec-list-net")
	withViolations := env.createExtClient(t, "ec-list-client-1")
	env.createExtClient(t, "ec-list-client-2")

	_extclient := &schema.Extclient{
		ID:                                withViolations.ID,
		TenantID:                          defaultTenantID,
		PostureCheckSeverity:              schema.SeverityHigh,
		PostureCheckLastEvaluationCycleID: uuid.NewString(),
	}
	require.NoError(t, _extclient.UpsertViolations(env.ctx, []schema.PostureCheckViolation{
		{CheckID: "check-1", Name: "os", Severity: schema.SeverityHigh},
	}))

	rec := env.do(getNetworkExtClients, http.MethodGet, map[string]string{"network": env.network.Name}, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var got []models.ExtClient
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&got))
	require.Len(t, got, 2)
	for _, extclient := range got {
		assert.Empty(t, extclient.PrivateKey)
		if extclient.ID == withViolations.ID {
			require.Len(t, extclient.PostureChecksViolations, 1)
			assert.Equal(t, "check-1", extclient.PostureChecksViolations[0].CheckID)
		} else {
			assert.Empty(t, extclient.PostureChecksViolations)
		}
	}
}
