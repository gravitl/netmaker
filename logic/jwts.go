package logic

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/gravitl/netmaker/scope"
	"gorm.io/gorm"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/servercfg"
)

var jwtSecretKey []byte

// SetJWTSecret - sets the jwt secret on server startup
func SetJWTSecret() {
	currentSecret, jwtErr := GetJwtSecretValue()
	if jwtErr != nil {
		newValue := RandomString(64)
		jwtSecretKey = []byte(newValue) // 512 bit random password
		if err := StoreJWTSecret(string(jwtSecretKey)); err != nil {
			logger.FatalLog("something went wrong when configuring JWT authentication")
		}
	} else {
		jwtSecretKey = []byte(currentSecret)
	}
}

// CreateJWT func will used to create the JWT while signing in and signing out
func CreateJWT(uuid string, macAddress string, network string) (response string, err error) {
	expirationTime := time.Now().Add(15 * time.Minute)
	claims := &models.Claims{
		ID:         uuid,
		Network:    network,
		MacAddress: macAddress,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "Netmaker",
			Subject:   fmt.Sprintf("node|%s", uuid),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(expirationTime),
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
func CreateUserAccessJwtToken(ctx context.Context, username string, role schema.UserRoleID, d time.Time, tokenID string) (response string, err error) {
	claims := &models.UserClaims{
		Scope:     scope.Level(ctx),
		ScopeID:   scope.ID(ctx),
		UserName:  username,
		Role:      role,
		TokenType: models.AccessTokenType,
		Api:       servercfg.GetAPIHost(),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "Netmaker",
			Subject:   fmt.Sprintf("user|%s", username),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(d),
			ID:        tokenID,
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
func CreateUserJWT(ctx context.Context, username string, role schema.UserRoleID, appName string) (response string, err error) {
	duration := GetJwtValidityDuration()
	if appName == NetclientApp || appName == NetmakerDesktopApp {
		duration = GetJwtValidityDurationForClients()
	}

	expirationTime := time.Now().Add(duration)
	claims := &models.UserClaims{
		Scope:     scope.Level(ctx),
		ScopeID:   scope.ID(ctx),
		UserName:  username,
		Role:      role,
		TokenType: models.UserIDTokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "Netmaker",
			Subject:   fmt.Sprintf("user|%s", username),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtSecretKey)
	if err == nil {
		return tokenString, nil
	}
	return "", err
}

// CreatePreAuthToken generate a jwt token to be used as intermediate
// token after primary-factor authentication but before secondary-factor
// authentication.
func CreatePreAuthToken(username string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    "Netmaker",
		Subject:   username,
		Audience:  []string{"auth:mfa"},
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
	})

	return token.SignedString(jwtSecretKey)
}

func GenerateOTPAuthURLSignature(url string) string {
	signer := hmac.New(sha256.New, jwtSecretKey)
	signer.Write([]byte(url))
	return hex.EncodeToString(signer.Sum(nil))
}

func VerifyOTPAuthURL(url, signature string) bool {
	signatureBytes, err := hex.DecodeString(signature)
	if err != nil {
		return false
	}

	signer := hmac.New(sha256.New, jwtSecretKey)
	signer.Write([]byte(url))
	return hmac.Equal(signatureBytes, signer.Sum(nil))
}

func GetUserNameFromToken(ctx context.Context, authtoken string) (username string, err error) {
	claims := &models.UserClaims{}
	var tokenSplit = strings.Split(authtoken, " ")
	var tokenString = ""

	if len(tokenSplit) < 2 {
		return "", Unauthorized_Err
	} else {
		tokenString = tokenSplit[1]
	}
	if tokenString == servercfg.GetMasterKey() && servercfg.GetMasterKey() != "" {
		return MasterUser, nil
	}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtSecretKey, nil
	})
	if err != nil {
		return "", Unauthorized_Err
	}

	for _, aud := range claims.Audience {
		// token created for mfa cannot be used for
		// anything else.
		if aud == "auth:mfa" {
			return "", Unauthorized_Err
		}
	}

	if claims.TokenType == models.AccessTokenType {
		jti := claims.ID
		if jti != "" {
			a := schema.UserAccessToken{ID: jti}
			// check if access token is active
			err := a.Get(ctx)
			if err != nil {
				err = errors.New("token revoked")
				return "", err
			}
			a.LastUsed = time.Now().UTC()
			a.Update(ctx)
		}
	}

	if token != nil && token.Valid {
		// check that user exists
		user := &schema.User{Username: claims.UserName}
		err = user.Get(ctx)
		if err != nil {
			return "", err
		}

		err = checkUserAccess(ctx, user.ID, claims)
		if err != nil {
			return "", err
		}

		return user.Username, Unauthorized_Err
	}

	return "", Unauthorized_Err
}

// VerifyUserToken func will used to Verify the JWT Token while using APIS
func VerifyUserToken(ctx context.Context, tokenString string) (username string, issuperadmin, isadmin bool, err error) {
	claims := &models.UserClaims{}

	if tokenString == servercfg.GetMasterKey() && servercfg.GetMasterKey() != "" {
		return MasterUser, true, true, nil
	}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtSecretKey, nil
	})
	if claims.TokenType == models.AccessTokenType {
		jti := claims.ID
		if jti != "" {
			a := schema.UserAccessToken{ID: jti}
			// check if access token is active
			err := a.Get(ctx)
			if err != nil {
				err = errors.New("token revoked")
				return "", false, false, err
			}
			a.LastUsed = time.Now().UTC()
			a.Update(ctx)
		}
	}
	if token != nil && token.Valid {
		// check that user exists
		user := &schema.User{Username: claims.UserName}
		err = user.Get(ctx)
		if err != nil {
			return "", false, false, err
		}

		err = checkUserAccess(ctx, user.ID, claims)
		if err != nil {
			return "", false, false, err
		}

		return user.Username, user.PlatformRoleID == schema.SuperAdminRole,
			user.PlatformRoleID == schema.AdminRole, nil
	}
	return "", false, false, err
}

func checkUserAccess(ctx context.Context, userID string, claims *models.UserClaims) error {
	if scope.Level(ctx) == scope.OrgScope {
		membership := &schema.OrgMembership{
			OrganizationID: scope.ID(ctx),
			UserID:         userID,
		}
		err := membership.Get(ctx)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return Unauthorized_Err
			}

			return err
		}

		if claims.Scope == scope.OrgScope && claims.ScopeID == scope.ID(ctx) && claims.Role == membership.RoleID {
			return nil
		}

		return Unauthorized_Err
	} else if scope.Level(ctx) == scope.TenantScope {
		membership := &schema.TenantMembership{
			TenantID: scope.ID(ctx),
			UserID:   userID,
		}
		err := membership.Get(ctx)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return Unauthorized_Err
			}

			return err
		}

		if claims.Scope == scope.OrgScope {
			tenant := &schema.Tenant{
				ID: scope.ID(ctx),
			}
			err = tenant.Get(ctx)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return Unauthorized_Err
				}

				return err
			}

			if claims.ScopeID == tenant.OrganizationID {
				return nil
			}

			return Unauthorized_Err
		} else if claims.Scope == scope.TenantScope && claims.ScopeID == scope.ID(ctx) && claims.Role == membership.RoleID {
			return nil
		}
	}

	return Unauthorized_Err
}

// VerifyHostToken - [hosts] Only
func VerifyHostToken(ctx context.Context, tokenString string) (hostID string, mac string, network string, err error) {
	claims := &models.Claims{}

	// this may be a stupid way of serving up a master key
	// TODO: look into a different method. Encryption?
	if tokenString == servercfg.GetMasterKey() && servercfg.GetMasterKey() != "" {
		return MasterUser, "", "", nil
	}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtSecretKey, nil
	})

	if token != nil && token.Valid {
		if !strings.HasPrefix(claims.Subject, "node|") {
			return "", "", "", errors.New("not a host token")
		}
		if claims.ID == "" {
			return "", "", "", errors.New("invalid host token: missing host ID")
		}

		hostID, err := uuid.Parse(claims.ID)
		if err != nil {
			return "", "", "", errors.New("invalid host token: invalid host ID")
		}

		host := &schema.Host{
			ID: hostID,
		}
		err = host.Get(ctx)
		if err != nil {
			return "", "", "", fmt.Errorf("error getting host %s: %w", claims.ID, err)
		}

		if scope.ID(ctx) != host.TenantID {
			return "", "", "", fmt.Errorf("host %s does not belong to tenant %s", claims.ID, host.TenantID)
		}
		return claims.ID, claims.MacAddress, claims.Network, nil
	}
	return "", "", "", err
}
