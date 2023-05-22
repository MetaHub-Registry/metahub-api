package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/metahub-registry/metahub-core/pkg/helper"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/types/www"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

const (
	pkgNameUser = "hanlders/user.go"
)

func (h Handlers) UserGetOne(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := chi.URLParam(r, "id")
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameUser,
		"func":    "UserGetOne",
		"userID":  userID,
	})
	_ = l
	defer span.End()
	if uID, err := strconv.Atoi(userID); err != nil {
		utils.NewErrorPlainWWW(w, l, span, "BAD_REQUEST", "failed to get user with uid '%s' (not an int?)", userID)
		return
	} else {
		utils.Debug(l, span, "Get user with id '%d'", uID)
		sqlUsr, e := h.mag.UserManagement.GetOne(ctx, uID)
		if e != nil {
			utils.ErrorHandlerPlainWWW(w, l, span, e, "INTERNAL_ERROR", "failed to get user with uid '%d'", uID)
			return
		}
		wwwUsr := sqlUsr.ToResponse()
		wwwByte := wwwUsr.ToBytes()
		helper.WriteResponse(w, http.StatusOK, wwwByte)
	}
}

func (h Handlers) UserPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameUser,
		"func":    "UserPost",
	})
	defer span.End()
	buf, err := io.ReadAll(r.Body)
	if err != nil {
		log.Fatal("request", err)
	}
	var wwwUser www.User
	if e := json.Unmarshal(buf, &wwwUser); e != nil {
		utils.ErrorHandlerPlainWWW(w, l, span, e, "INTERNAL_ERROR", "failed to unmarshal request")
		return
	}
	utils.Debug(l, span, "Got user '%s'", wwwUser.GithubUsername)
	if found, e := h.mag.UserManagement.UserNameExists(ctx, wwwUser.GithubUsername); e != nil {
		utils.ErrorHandlerPlainWWW(w, l, span, e, "INTERNAL_ERROR", "failed to check if user '%s' exists", wwwUser.GithubUsername)
		return
	} else if found {
		utils.ErrorHandlerPlainWWW(w, l, span, e, "BAD_REQUEST", "user '%s' already exists", wwwUser.GithubUsername)
		return
	}

	if uID, e := h.mag.UserManagement.CreateWWWUser(ctx, wwwUser); e != nil {
		utils.ErrorHandlerPlainWWW(w, l, span, e, "INTERNAL_ERROR", "failed to insert user")
		return
	} else {
		sqlUsr, e := h.mag.UserManagement.GetOne(ctx, int(uID))
		if e != nil {
			utils.ErrorHandlerPlainWWW(w, l, span, e, "INTERNAL_ERROR", "failed to get user with uid '%d'", uID)
			return
		}
		wwwUser = sqlUsr.ToResponse()
		wwwByte := wwwUser.ToBytes()
		helper.WriteResponse(w, http.StatusCreated, wwwByte)
		return
	}
}

func (h Handlers) UserGetSession(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameUser,
		"func":    "UserGetSession",
	})
	_ = l
	defer span.End()
	cookie, err := r.Cookie("loginName")
	if err != nil {
		switch {
		case errors.Is(err, http.ErrNoCookie):
			http.Error(w, "cookie not found", http.StatusBadRequest)
		default:
			log.Println(err)
			http.Error(w, "server error", http.StatusInternalServerError)
		}
		return
	}
	sqlUser, err := h.mag.UserManagement.GetByUsername(ctx, cookie.Value)
	if err != nil {
		utils.ErrorHandlerPlainWWW(w, l, span, err, "INTERNAL_ERROR", "failed to get user '%s'", cookie.Value)
		return
	}
	wwwSession := www.Session{
		ID:     sqlUser.GithubID,
		Login:  sqlUser.UserName,
		Avatar: sqlUser.AvatarURL,
	}
	wwwByte := wwwSession.ToBytes()
	helper.WriteResponse(w, http.StatusOK, wwwByte)
}

func (h Handlers) UserPutSecret(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := chi.URLParam(r, "id")
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameUser,
		"func":    "UserPutSecret",
		"userID":  userID,
	})
	_ = l
	defer span.End()
	_ = ctx
	if uID, err := strconv.Atoi(userID); err != nil {
		utils.NewErrorPlainWWW(w, l, span, "BAD_REQUEST", "failed to get user with uid '%s' (not an int?)", userID)
		return
	} else {
		utils.Debug(l, span, "Get user with id '%d'", uID)
		sqlUsr, e := h.mag.UserManagement.GetOne(ctx, uID)
		if e != nil {
			utils.ErrorHandlerPlainWWW(w, l, span, e, "INTERNAL_ERROR", "failed to get user with uid '%d'", uID)
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
		helper.WriteResponse(w, http.StatusCreated, nil)
	}
}
