package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/metahub-registry/metahub-core/pkg/backends/sqlite"
	"github.com/metahub-registry/metahub-core/pkg/helper"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/types/www"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

const (
	pkgNameUser = "hanlders/user.go"
)

func getUserClaim(ctx context.Context, r *http.Request) (uname, ghid string, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameUser,
		"func":    "getUserClaim",
	})
	defer span.End()
	_ = ctx
	if _, claims, e := jwtauth.FromContext(r.Context()); e != nil {
		return "", "", utils.ErrorHandler(l, span, e, "failed to get claims")
	} else if usr, ok := claims["user"]; !ok {
		err = utils.NewError(l, span, "failed to get user from claims")
		return "", "", err

	} else {
		utils.Debug(l, span, "Got user '%v'", claims)
		uname = usr.(string)
		if val, ok := claims["id"]; ok {
			ghid = val.(string)
		} else {
			err = utils.NewError(l, span, "failed to get ID from claims")
			return "", "", err
		}
		utils.Debug(l, span, "Got username '%s' and ghid '%s' from claims", uname, ghid)
	}
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
		utils.ErrorHandlerPlainWWW(w, l, span, err, "INTERNAL_ERROR", "failed to get user")
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
	userName, ghID, err := getUserClaim(ctx, r)
	if err != nil {
		return
	}
	utils.Debug(l, span, "User '%s' (GHID: %s) attempts to get his user", userName, ghID)
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
		utils.ErrorHandlerPlainWWW(w, l, span, err, "INTERNAL_ERROR", "failed to get user")
	} else if isLoggedIn {
		wwwSession := www.Session{
			ID:     sqlUser.GithubID,
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
		utils.ErrorHandlerPlainWWW(w, l, span, err, "INTERNAL_ERROR", "failed to get user")
	}
	if !isLoggedIn {
		// error was already written
		return
	}
	// TODO: Add Secrets to userID
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
	secretID := chi.URLParam(r, "id")
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameUser,
		"func":    "UserDelSecret",
		"id":      secretID,
	})
	defer span.End()
	_ = ctx
	sqlUsr, isLoggedIn, err := h.userCheckLoginGetUser(ctx, w, r)
	if err != nil {
		utils.ErrorHandlerPlainWWW(w, l, span, err, "INTERNAL_ERROR", "failed to get user")
	}
	if !isLoggedIn {
		// error was already written
		return
	}
	_, err = strconv.Atoi(secretID)
	if err == nil {
		utils.Debug(l, span, "Deleting secret by ID '%s'", secretID)
		if err := h.mag.UserManagement.DelSecret(ctx, sqlUsr.ID, secretID); err != nil {
			utils.ErrorHandlerPlainWWW(w, l, span, err, "INTERNAL_ERROR", "failed to delete secret")
			return
		}
		h.writeJSON(w, http.StatusOK, envelope{"message": "deleted secret by ID, please transition to secret name"})
		return
	}
	utils.Debug(l, span, "Deleting secret by name '%s'", secretID)
	if err := h.mag.UserManagement.DelSecretByName(ctx, sqlUsr.ID, secretID); err != nil {
		utils.ErrorHandlerPlainWWW(w, l, span, err, "INTERNAL_ERROR", "failed to delete secret")
		return
	}
	helper.WriteResponse(w, http.StatusOK, nil)
}
