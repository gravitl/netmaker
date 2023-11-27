package migrate

import (
	"encoding/json"
	"log"

	"golang.org/x/exp/slog"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
)

// Run - runs all migrations
func Run() {
	updateEnrollmentKeys()
	assignSuperAdmin()
	updateHosts()
}

func assignSuperAdmin() {
	users, err := logic.GetUsers()
	if err != nil || len(users) == 0 {
		return
	}

	if ok, _ := logic.HasSuperAdmin(); ok {
		return
	}
	createdSuperAdmin := false
	owner := servercfg.GetOwnerEmail()
	if owner != "" {
		user, err := logic.GetUser(owner)
		if err != nil {
			log.Fatal("error getting user", "user", owner, "error", err.Error())
		}
		user.IsSuperAdmin = true
		user.IsAdmin = false
		err = logic.UpsertUser(*user)
		if err != nil {
			log.Fatal(
				"error updating user to superadmin",
				"user",
				user.UserName,
				"error",
				err.Error(),
			)
		}
		return
	}
	for _, u := range users {
		if u.IsAdmin {
			user, err := logic.GetUser(u.UserName)
			if err != nil {
				slog.Error("error getting user", "user", u.UserName, "error", err.Error())
				continue
			}
			user.IsSuperAdmin = true
			user.IsAdmin = false
			err = logic.UpsertUser(*user)
			if err != nil {
				slog.Error(
					"error updating user to superadmin",
					"user",
					user.UserName,
					"error",
					err.Error(),
				)
				continue
			} else {
				createdSuperAdmin = true
			}
			break
		}
	}

	if !createdSuperAdmin {
		slog.Error("failed to create superadmin!!")
	}
}

func updateEnrollmentKeys() {
	rows, err := database.FetchRecords(database.ENROLLMENT_KEYS_TABLE_NAME)
	if err != nil {
		return
	}
	for _, row := range rows {
		var key models.EnrollmentKey
		if err = json.Unmarshal([]byte(row), &key); err != nil {
			continue
		}
		if key.Type != models.Undefined {
			logger.Log(2, "migration: enrollment key type already set")
			continue
		} else {
			logger.Log(2, "migration: updating enrollment key type")
			if key.Unlimited {
				key.Type = models.Unlimited
			} else if key.UsesRemaining > 0 {
				key.Type = models.Uses
			} else if !key.Expiration.IsZero() {
				key.Type = models.TimeExpiration
			}
		}
		data, err := json.Marshal(key)
		if err != nil {
			logger.Log(0, "migration: marshalling enrollment key: "+err.Error())
			continue
		}
		if err = database.Insert(key.Value, string(data), database.ENROLLMENT_KEYS_TABLE_NAME); err != nil {
			logger.Log(0, "migration: inserting enrollment key: "+err.Error())
			continue
		}

	}
}

func updateHosts() {
	rows, err := database.FetchRecords(database.HOSTS_TABLE_NAME)
	if err != nil {
		logger.Log(0, "failed to fetch database records for hosts")
	}
	for _, row := range rows {
		var host models.Host
		if err := json.Unmarshal([]byte(row), &host); err != nil {
			logger.Log(0, "failed to unmarshal database row to host", "row", row)
			continue
		}
		if host.PersistentKeepalive == 0 {
			host.PersistentKeepalive = models.DefaultPersistentKeepAlive
			if err := logic.UpsertHost(&host); err != nil {
				logger.Log(0, "failed to upsert host", host.ID.String())
				continue
			}
		}
	}
}
