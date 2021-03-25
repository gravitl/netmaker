package functions

import (
    "time"
    "github.com/gravitl/netmaker/config"
    "github.com/gravitl/netmaker/models"
    "github.com/dgrijalva/jwt-go"
)

var jwtSecretKey = []byte("(BytesOverTheWire)")

// CreateJWT func will used to create the JWT while signing in and signing out
func CreateJWT(macaddress string, group string) (response string, err error) {
    expirationTime := time.Now().Add(5 * time.Minute)
    claims := &models.Claims{
        MacAddress: macaddress,
        Group: group,
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

func CreateUserJWT(username string, isadmin bool) (response string, err error) {
    expirationTime := time.Now().Add(60 * time.Minute)
    claims := &models.UserClaims{
        UserName: username,
        IsAdmin: isadmin,
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
func VerifyUserToken(tokenString string) (username string, isadmin bool, err error) {
    claims := &models.UserClaims{}

    token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
        return jwtSecretKey, nil
    })

    if token != nil {
        return claims.UserName, claims.IsAdmin, nil
    }
    return "", false, err
}

// VerifyToken func will used to Verify the JWT Token while using APIS
func VerifyToken(tokenString string) (macaddress string, group string, err error) {
    claims := &models.Claims{}

    //this may be a stupid way of serving up a master key
    //TODO: look into a different method. Encryption?
    if tokenString == config.Config.Server.MasterKey {
        return "mastermac", "", nil
    }

    token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
        return jwtSecretKey, nil
    })

    if token != nil {
        return claims.MacAddress, claims.Group, nil
    }
    return "", "", err
}

