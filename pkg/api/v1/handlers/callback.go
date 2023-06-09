package handlers

import (
	"context"

	"github.com/go-chi/jwtauth/v5"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

const (
	pkgNameCB   = "hanlders/callback.go"
	tokenSecret = "secret"
)

var TokenAuth *jwtauth.JWTAuth

func init() {
	TokenAuth = jwtauth.New("HS256", []byte(tokenSecret), nil)
}

func (h Handlers) CreateToken(ctx context.Context, usrName, userID string) (string, error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameCB,
		"func":    "CreateToken",
	})
	defer span.End()
	_ = ctx
	utils.Debug(l, span, "Create token for user '%s'", usrName)
	_, tokenString, err := TokenAuth.Encode(map[string]interface{}{
		"user": usrName,
		"id":   userID,
	})
	return tokenString, err
}

func (h Handlers) CreateJWT(ctx context.Context, usrName, userID string) (t jwt.Token, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameCB,
		"func":    "CreateJWT",
	})
	defer span.End()
	_ = ctx
	utils.Debug(l, span, "Create token for user '%s'", usrName)
	t, _, err = TokenAuth.Encode(map[string]interface{}{
		"user": usrName,
		"id":   userID,
	})
	return t, err
}
