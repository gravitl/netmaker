package logic

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
)

var jwtSecretKey = []byte("(BytesOverTheWire)")

// CreateJWT func will used to create the JWT while signing in and signing out
func CreateJWT(macaddress string, network string) (response string, err error) {
	expirationTime := time.Now().Add(5 * time.Minute)
	claims := &models.Claims{
		MacAddress: macaddress,
		Network:    network,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtSecretKey)
	if err == nil {
		return tokenString, nil
	}
	return "", err
}

// CreateUserJWT - creates a user jwt token
func CreateUserJWT(username string, networks []string, isadmin bool) (response string, err error) {
	expirationTime := time.Now().Add(60 * 12 * time.Minute)
	claims := &models.UserClaims{
		UserName: username,
		Networks: networks,
		IsAdmin:  isadmin,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtSecretKey)
	if err == nil {
		return tokenString, nil
	}
	return "", err
}

// VerifyToken func will used to Verify the JWT Token while using APIS
func VerifyUserToken(tokenString string) (username string, networks []string, isadmin bool, err error) {
	claims := &models.UserClaims{}

	if tokenString == servercfg.GetMasterKey() {
		return "masteradministrator", nil, true, nil
	}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtSecretKey, nil
	})

	if token != nil && token.Valid {
		// check that user exists
		if user, err := GetUser(claims.UserName); user.UserName != "" && err == nil {
			return claims.UserName, claims.Networks, claims.IsAdmin, nil
		}
		err = errors.New("user does not exist")
	}
	return "", nil, false, err
}

// VerifyToken - gRPC [nodes] Only
func VerifyToken(tokenString string) (macaddress string, network string, err error) {
	claims := &models.Claims{}

	//this may be a stupid way of serving up a master key
	//TODO: look into a different method. Encryption?
	if tokenString == servercfg.GetMasterKey() {
		return "mastermac", "", nil
	}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtSecretKey, nil
	})

	if token != nil {
		return claims.MacAddress, claims.Network, nil
	}
	return "", "", err
}
