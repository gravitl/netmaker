package logic

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"
	"golang.org/x/crypto/bcrypt"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic/pro"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/models/promodels"
	"github.com/gravitl/netmaker/servercfg"
)

// HasAdmin - checks if server has an admin
func HasAdmin() (bool, error) {

	collection, err := database.FetchRecords(database.USERS_TABLE_NAME)
	if err != nil {
		if database.IsEmptyRecord(err) {
			return false, nil
		} else {
			return true, err
		}
	}
	for _, value := range collection { // filter for isadmin true
		var user models.User
		err = json.Unmarshal([]byte(value), &user)
		if err != nil {
			continue
		}
		if user.IsAdmin {
			return true, nil
		}
	}

	return false, err
}

// GetReturnUser - gets a user
func GetReturnUser(username string) (models.ReturnUser, error) {

	var user models.ReturnUser
	record, err := database.FetchRecord(database.USERS_TABLE_NAME, username)
	if err != nil {
		return user, err
	}
	if err = json.Unmarshal([]byte(record), &user); err != nil {
		return models.ReturnUser{}, err
	}
	return user, err
}

// GetUsers - gets users
func GetUsers() ([]models.ReturnUser, error) {

	var users []models.ReturnUser

	collection, err := database.FetchRecords(database.USERS_TABLE_NAME)

	if err != nil {
		return users, err
	}

	for _, value := range collection {

		var user models.ReturnUser
		err = json.Unmarshal([]byte(value), &user)
		if err != nil {
			continue // get users
		}
		users = append(users, user)
	}

	return users, err
}

// CreateUser - creates a user
func CreateUser(user *models.User) error {
	// check if user exists
	if _, err := GetUser(user.UserName); err == nil {
		return errors.New("user exists")
	}
	var err = ValidateUser(user)
	if err != nil {
		return err
	}

	// encrypt that password so we never see it again
	hash, err := bcrypt.GenerateFromPassword([]byte(user.Password), 5)
	if err != nil {
		return err
	}
	// set password to encrypted password
	user.Password = string(hash)

	tokenString, _ := CreateProUserJWT(user.UserName, user.Networks, user.Groups, user.IsAdmin)
	if tokenString == "" {
		// logic.ReturnErrorResponse(w, r, errorResponse)
		return err
	}

	SetUserDefaults(user)

	// connect db
	data, err := json.Marshal(user)
	if err != nil {
		return err
	}
	err = database.Insert(user.UserName, string(data), database.USERS_TABLE_NAME)
	if err != nil {
		return err
	}

	// == PRO == Add user to every network as network user ==
	currentNets, err := GetNetworks()
	if err != nil {
		currentNets = []models.Network{}
	}
	for i := range currentNets {
		newUser := promodels.NetworkUser{
			ID:      promodels.NetworkUserID(user.UserName),
			Clients: []string{},
			Nodes:   []string{},
		}

		pro.AddProNetDefaults(&currentNets[i])
		if pro.IsUserAllowed(&currentNets[i], user.UserName, user.Groups) {
			newUser.AccessLevel = currentNets[i].ProSettings.DefaultAccessLevel
			newUser.ClientLimit = currentNets[i].ProSettings.DefaultUserClientLimit
			newUser.NodeLimit = currentNets[i].ProSettings.DefaultUserNodeLimit
		} else {
			newUser.AccessLevel = pro.NO_ACCESS
			newUser.ClientLimit = 0
			newUser.NodeLimit = 0
		}

		// legacy
		if StringSliceContains(user.Networks, currentNets[i].NetID) {
			if !servercfg.Is_EE {
				newUser.AccessLevel = pro.NET_ADMIN
			}
		}
		userErr := pro.CreateNetworkUser(&currentNets[i], &newUser)
		if userErr != nil {
			logger.Log(0, "failed to add network user data on network", currentNets[i].NetID, "for user", user.UserName)
		}
	}
	// == END PRO ==

	return nil
}

// CreateAdmin - creates an admin user
func CreateAdmin(admin *models.User) error {
	hasadmin, err := HasAdmin()
	if err != nil {
		return err
	}
	if hasadmin {
		return errors.New("admin user already exists")
	}
	admin.IsAdmin = true
	return CreateUser(admin)
}

// VerifyAuthRequest - verifies an auth request
func VerifyAuthRequest(authRequest models.UserAuthParams) (string, error) {
	var result models.User
	if authRequest.UserName == "" {
		return "", errors.New("username can't be empty")
	} else if authRequest.Password == "" {
		return "", errors.New("password can't be empty")
	}
	// Search DB for node with Mac Address. Ignore pending nodes (they should not be able to authenticate with API until approved).
	record, err := database.FetchRecord(database.USERS_TABLE_NAME, authRequest.UserName)
	if err != nil {
		return "", errors.New("error retrieving user from db: " + err.Error())
	}
	if err = json.Unmarshal([]byte(record), &result); err != nil {
		return "", errors.New("error unmarshalling user json: " + err.Error())
	}

	// compare password from request to stored password in database
	// might be able to have a common hash (certificates?) and compare those so that a password isn't passed in in plain text...
	// TODO: Consider a way of hashing the password client side before sending, or using certificates
	if err = bcrypt.CompareHashAndPassword([]byte(result.Password), []byte(authRequest.Password)); err != nil {
		return "", errors.New("incorrect credentials")
	}

	// Create a new JWT for the node
	tokenString, _ := CreateProUserJWT(authRequest.UserName, result.Networks, result.Groups, result.IsAdmin)
	return tokenString, nil
}

// UpdateUserNetworks - updates the networks of a given user
func UpdateUserNetworks(newNetworks, newGroups []string, isadmin bool, currentUser *models.ReturnUser) error {
	// check if user exists
	returnedUser, err := GetUser(currentUser.UserName)
	if err != nil {
		return err
	} else if returnedUser.IsAdmin {
		return fmt.Errorf("can not make changes to an admin user, attempted to change %s", returnedUser.UserName)
	}
	if isadmin {
		currentUser.IsAdmin = true
		currentUser.Networks = nil
	} else {
		// == PRO ==
		currentUser.Groups = newGroups
		for _, n := range newNetworks {
			if !StringSliceContains(currentUser.Networks, n) {
				// make net admin of any network not previously assigned
				pro.MakeNetAdmin(n, currentUser.UserName)
			}
		}
		// Compare networks, find networks not in previous
		for _, n := range currentUser.Networks {
			if !StringSliceContains(newNetworks, n) {
				// if user was removed from a network, re-assign access to net default level
				if network, err := GetNetwork(n); err == nil {
					if network.ProSettings != nil {
						ok := pro.AssignAccessLvl(n, currentUser.UserName, network.ProSettings.DefaultAccessLevel)
						if ok {
							logger.Log(0, "changed", currentUser.UserName, "access level on network", network.NetID, "to", fmt.Sprintf("%d", network.ProSettings.DefaultAccessLevel))
						}
					}
				}
			}
		}

		if err := AdjustGroupPermissions(currentUser); err != nil {
			logger.Log(0, "failed to update user", currentUser.UserName, "after group update", err.Error())
		}
		// == END PRO ==

		currentUser.Networks = newNetworks
	}

	userChange := models.User{
		UserName: currentUser.UserName,
		Networks: currentUser.Networks,
		IsAdmin:  currentUser.IsAdmin,
		Password: "",
		Groups:   currentUser.Groups,
	}

	_, err = UpdateUser(&userChange, returnedUser)

	return err
}

// UpdateUser - updates a given user
func UpdateUser(userchange, user *models.User) (*models.User, error) {
	// check if user exists
	if _, err := GetUser(user.UserName); err != nil {
		return &models.User{}, err
	}

	queryUser := user.UserName

	if userchange.UserName != "" {
		user.UserName = userchange.UserName
	}
	if len(userchange.Networks) > 0 {
		user.Networks = userchange.Networks
	}
	if len(userchange.Groups) > 0 {
		user.Groups = userchange.Groups
	}
	if userchange.Password != "" {
		// encrypt that password so we never see it again
		hash, err := bcrypt.GenerateFromPassword([]byte(userchange.Password), 5)

		if err != nil {
			return userchange, err
		}
		// set password to encrypted password
		userchange.Password = string(hash)

		user.Password = userchange.Password
	}

	if (userchange.IsAdmin != user.IsAdmin) && !user.IsAdmin {
		user.IsAdmin = userchange.IsAdmin
	}

	err := ValidateUser(user)
	if err != nil {
		return &models.User{}, err
	}

	if err = database.DeleteRecord(database.USERS_TABLE_NAME, queryUser); err != nil {
		return &models.User{}, err
	}
	data, err := json.Marshal(&user)
	if err != nil {
		return &models.User{}, err
	}
	if err = database.Insert(user.UserName, string(data), database.USERS_TABLE_NAME); err != nil {
		return &models.User{}, err
	}
	logger.Log(1, "updated user", queryUser)
	return user, nil
}

// ValidateUser - validates a user model
func ValidateUser(user *models.User) error {

	v := validator.New()
	_ = v.RegisterValidation("in_charset", func(fl validator.FieldLevel) bool {
		isgood := user.NameInCharSet()
		return isgood
	})
	err := v.Struct(user)

	if err != nil {
		for _, e := range err.(validator.ValidationErrors) {
			logger.Log(2, e.Error())
		}
	}

	return err
}

// DeleteUser - deletes a given user
func DeleteUser(user string) (bool, error) {

	if userRecord, err := database.FetchRecord(database.USERS_TABLE_NAME, user); err != nil || len(userRecord) == 0 {
		return false, errors.New("user does not exist")
	}

	err := database.DeleteRecord(database.USERS_TABLE_NAME, user)
	if err != nil {
		return false, err
	}

	// == pro - remove user from all network user instances ==
	currentNets, err := GetNetworks()
	if err != nil {
		if database.IsEmptyRecord(err) {
			currentNets = []models.Network{}
		} else {
			return true, err
		}
	}

	for i := range currentNets {
		netID := currentNets[i].NetID
		if err = pro.DeleteNetworkUser(netID, user); err != nil {
			logger.Log(0, "failed to remove", user, "as network user from network", netID, err.Error())
		}
	}

	return true, nil
}

// FetchAuthSecret - manages secrets for oauth
func FetchAuthSecret(key string, secret string) (string, error) {
	var record, err = database.FetchRecord(database.GENERATED_TABLE_NAME, key)
	if err != nil {
		if err = database.Insert(key, secret, database.GENERATED_TABLE_NAME); err != nil {
			return "", err
		} else {
			return secret, nil
		}
	}
	return record, nil
}

// GetState - gets an SsoState from DB, if expired returns error
func GetState(state string) (*models.SsoState, error) {
	var s models.SsoState
	record, err := database.FetchRecord(database.SSO_STATE_CACHE, state)
	if err != nil {
		return &s, err
	}

	if err = json.Unmarshal([]byte(record), &s); err != nil {
		return &s, err
	}

	if s.IsExpired() {
		return &s, fmt.Errorf("state expired")
	}

	return &s, nil
}

// SetState - sets a state with new expiration
func SetState(state string) error {
	s := models.SsoState{
		Value:      state,
		Expiration: time.Now().Add(models.DefaultExpDuration),
	}

	data, err := json.Marshal(&s)
	if err != nil {
		return err
	}

	return database.Insert(state, string(data), database.SSO_STATE_CACHE)
}

// IsStateValid - checks if given state is valid or not
// deletes state after call is made to clean up, should only be called once per sign-in
func IsStateValid(state string) (string, bool) {
	s, err := GetState(state)
	if err != nil {
		logger.Log(2, "error retrieving oauth state:", err.Error())
		return "", false
	}
	if s.Value != "" {
		if err = delState(state); err != nil {
			logger.Log(2, "error deleting oauth state:", err.Error())
			return "", false
		}
	}
	return s.Value, true
}

// delState - removes a state from cache/db
func delState(state string) error {
	return database.DeleteRecord(database.SSO_STATE_CACHE, state)
}

// PRO

// AdjustGroupPermissions - adjusts a given user's network access based on group changes
func AdjustGroupPermissions(user *models.ReturnUser) error {
	networks, err := GetNetworks()
	if err != nil {
		return err
	}
	// UPDATE
	// go through all networks and see if new group is in
	// if access level of current user is greater (value) than network's default
	// assign network's default
	// DELETE
	// if user not allowed on network a
	for i := range networks {
		AdjustNetworkUserPermissions(user, &networks[i])
	}

	return nil
}

// AdjustNetworkUserPermissions - adjusts a given user's network access based on group changes
func AdjustNetworkUserPermissions(user *models.ReturnUser, network *models.Network) error {
	networkUser, err := pro.GetNetworkUser(
		network.NetID,
		promodels.NetworkUserID(user.UserName),
	)
	if err == nil && network.ProSettings != nil {
		if pro.IsUserAllowed(network, user.UserName, user.Groups) {
			if networkUser.AccessLevel > network.ProSettings.DefaultAccessLevel {
				networkUser.AccessLevel = network.ProSettings.DefaultAccessLevel
			}
			if networkUser.NodeLimit < network.ProSettings.DefaultUserNodeLimit {
				networkUser.NodeLimit = network.ProSettings.DefaultUserNodeLimit
			}
			if networkUser.ClientLimit < network.ProSettings.DefaultUserClientLimit {
				networkUser.ClientLimit = network.ProSettings.DefaultUserClientLimit
			}
		} else {
			networkUser.AccessLevel = pro.NO_ACCESS
			networkUser.NodeLimit = 0
			networkUser.ClientLimit = 0
		}
		pro.UpdateNetworkUser(network.NetID, networkUser)
	}
	return err
}
