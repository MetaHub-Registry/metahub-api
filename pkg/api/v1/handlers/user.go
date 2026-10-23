package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/metahub-registry/metahub-core/pkg/backends/sqlite"
	"github.com/metahub-registry/metahub-core/pkg/helper"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/types/www"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

const (
	pkgNameUser      = "hanlders/user.go"
	errFailedGetUser = "failed to get user"
)

// getUserClaim extracts the username and OIDC sub from the request context.
// The context must have been populated by OIDCAuthMiddleware.
func getUserClaim(ctx context.Context, r *http.Request) (uname, sub string, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameUser,
		"func":    "getUserClaim",
	})
	defer span.End()
	_ = ctx
	claims, ok := OIDCClaimsFromContext(r.Context())
	if !ok || claims == nil {
		err = utils.NewError(l, span, "no OIDC claims in context")
		return
	}
	uname = claims.Username
	sub = claims.Sub
	utils.Debug(l, span, "Got username '%s' (sub: %s) from OIDC claims", uname, sub)
	return
}

func (h Handlers) UserGetOne(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameUser,
		"func":    "UserGetOne",
	})
	defer span.End()

	if sqlUsr, isLoggedIn, err := h.userCheckLoginGetUser(ctx, w, r); err != nil {
		utils.ErrorHandlerPlainWWW(w, l, span, err, "INTERNAL_ERROR", errFailedGetUser)
	} else if isLoggedIn {
		wwwUsr := sqlUsr.ToResponse()
		wwwByte := wwwUsr.ToBytes()
		helper.WriteResponse(w, http.StatusOK, wwwByte)
	}
}

func (h Handlers) userCheckLoginGetUser(ctx context.Context, w http.ResponseWriter, r *http.Request) (su *sqlite.User, isLoggedIn bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameUser,
		"func":    "userCheckLogin",
	})
	defer span.End()
	userName, sub, err := getUserClaim(ctx, r)
	if err != nil {
		return
	}
	utils.Debug(l, span, "User '%s' (sub: %s) attempts to get their user record", userName, sub)
	sqlUsr, err := h.mag.UserManagement.GetByUsername(ctx, userName)
	if err != nil {
		return
	}
	if sqlUsr.IsLoggedIn == 0 {
		e := fmt.Errorf("user '%s' is not logged in", sqlUsr.UserName)
		utils.ErrorHandlerPlainWWW(w, l, span, e, "UNAUTHORIZED", "")
		return
	}
	return sqlUsr, sqlUsr.IsLoggedIn == 1, nil
}

func (h Handlers) getUsername(ctx context.Context, r *http.Request) (uname string, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameUser,
		"func":    "getUsername",
	})
	defer span.End()
	_ = ctx
	uname, _, err = getUserClaim(ctx, r)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "failed to get user claim")
		return
	}
	return uname, nil
}

func (h Handlers) UserGetSession(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameUser,
		"func":    "UserGetSession",
	})
	utils.Debug(l, span, "Get session")
	defer span.End()
	if sqlUser, isLoggedIn, err := h.userCheckLoginGetUser(ctx, w, r); err != nil {
		utils.ErrorHandlerPlainWWW(w, l, span, err, "INTERNAL_ERROR", errFailedGetUser)
	} else if isLoggedIn {
		wwwSession := www.Session{
			ID:     sqlUser.OidcSub,
			Login:  sqlUser.UserName,
			Avatar: sqlUser.AvatarURL,
		}
		wwwByte := wwwSession.ToBytes()
		helper.WriteResponse(w, http.StatusOK, wwwByte)
	}
}

func (h Handlers) UserPutSecret(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameUser,
		"func":    "UserPutSecret",
	})
	defer span.End()
	_ = ctx
	sqlUsr, isLoggedIn, err := h.userCheckLoginGetUser(ctx, w, r)
	if err != nil {
		utils.ErrorHandlerPlainWWW(w, l, span, err, "INTERNAL_ERROR", errFailedGetUser)
	}
	if !isLoggedIn {
		// error was already written
		return
	}
	buf, err := io.ReadAll(r.Body)
	if err != nil {
		log.Fatal("request", err)
	}
	var wwwScrt www.Secret
	if e := json.Unmarshal(buf, &wwwScrt); e != nil {
		utils.ErrorHandlerPlainWWW(w, l, span, e, "INTERNAL_ERROR", "failed to unmarshal request")
		return
	}
	tScrt := types.SecretFromWWW(wwwScrt)
	if scrtID, err := h.mag.UserManagement.InsertSecret(ctx, int64(sqlUsr.ID), tScrt); err != nil {
		utils.ErrorHandlerPlainWWW(w, l, span, err, "INTERNAL_ERROR", "failed to insert secret")
		return
	} else {
		utils.Debug(l, span, "Inserted secret with id '%d'", scrtID)
	}
	h.writeJSON(w, http.StatusCreated, envelope{"name": wwwScrt.Name, "secret_key": wwwScrt.Key})

}

func (h Handlers) UserDelSecret(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	secretName := chi.URLParam(r, "name")
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameUser,
		"func":    "UserDelSecret",
		"name":    secretName,
	})
	defer span.End()
	_ = ctx
	sqlUsr, isLoggedIn, err := h.userCheckLoginGetUser(ctx, w, r)
	if err != nil {
		utils.ErrorHandlerPlainWWW(w, l, span, err, "INTERNAL_ERROR", errFailedGetUser)
	}
	if !isLoggedIn {
		// error was already written
		return
	}
	utils.Debug(l, span, "Deleting secret by name '%s'", secretName)
	if err := h.mag.UserManagement.DelSecretByName(ctx, sqlUsr.ID, secretName); err != nil {
		utils.ErrorHandlerPlainWWW(w, l, span, err, "INTERNAL_ERROR", "failed to delete secret")
		return
	}
	helper.WriteResponse(w, http.StatusOK, nil)
}
