package handlers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	gCtx "github.com/gorilla/context"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/helper"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

type oauthError string

const (
	defaultProfile = "generic"
	pkgNameAuth    = "api/auth.go"
	realFrmt       = "Realm: %s"
	uknAccTok      = "unknown access token"
)

var (
	invalidRequest oauthError = "invalid_request"
	invalidToken   oauthError = "invalid_token"
	userReg                   = regexp.MustCompile(`(?P<user>[\w\-\.\_]+)(\/(?P<profile>[\/\w\-\.\_#]+))?`)
	debugUser                 = map[string]int{"oci": 4}
	FailGetToken              = fmt.Errorf("failed to get token")
)

func getAccessTokenFromBearerToken(r *http.Request) (at *types.AccessToken, user string, err error) {
	at = &types.AccessToken{}
	authorizationHeader := r.Header.Get("authorization")
	if authorizationHeader == "" {
		return
	}
	bearerToken := strings.Split(authorizationHeader, " ")
	if len(bearerToken) != 2 {
		err = fmt.Errorf("invalid authorization header: %q", authorizationHeader)
		return
	}
	accessTokenString := bearerToken[1]
	decoded, _ := base64.StdEncoding.DecodeString(accessTokenString)
	parts := strings.Split(string(decoded), ":")
	if len(parts) != 2 {
		err = fmt.Errorf("invalid access token: %q", accessTokenString)
		return
	}
	user = parts[0]
	at.Token = accessTokenString
	return
}

func (h Handlers) AuthHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameAuth,
		"func":    "AuthHandler",
	})
	_ = l
	defer span.End()
	_ = ctx
	acct := r.URL.Query().Get("account")
	at, user, err := getAccessTokenFromBearerToken(r)
	if err != nil {
		err = errors.Wrap(err, "error getting access token from bearer token")
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	utils.Debug(l, span, "user:'%s' acct:'%s'", user, acct)

	if acct == "" && user == "" {
		at.Token = "REDACTED-IN-HISTORY"
	}
	//l.Debugf("User: %s // Token: %s", at.GetUser(), at.Token)
	// Prepare the body of the response
	responseBodyBytes := new(bytes.Buffer)
	w.Header().Add("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(responseBodyBytes).Encode(at)
	data := responseBodyBytes.Bytes()
	w.Write(data)
}

func (h Handlers) AuthMiddlewareHandler(w http.ResponseWriter, r *http.Request) (err error) {
	ctx := r.Context()
	req := types.NewRequest(ctx, r)
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameAuth,
		"func":    "AuthMiddlewareHandler",
		"host":    req.Host,
		"repo":    req.Repo,
		"tag":     req.Tag,
		"method":  req.Method,
		"digest":  req.Digest,
		"profile": req.Profile,
		"role":    req.UserRole,
	})
	defer span.End()
	var realm string
	switch r.Host {
	case "metahub-registry.org":
		realm = "https://metahub-registry.org"
	case "metahub.qnib.org":
		realm = "https://metahub.qnib.org"
	default:
		switch r.Proto {
		case "HTTP/1.0":
			realm = "http://" + r.Host
		case "HTTP/1.1":
			realm = "http://" + r.Host
		case "HTTP/2.0":
			realm = "https://" + r.Host
		}
	}
	//fmt.Printf("Request:$$$ %s\n", req.String())
	//dumpedReq, _ := httputil.DumpRequest(r, false)
	//fmt.Printf("Request:#### \n%s\n####", dumpedReq)
	authorizationHeader := r.Header.Get("authorization")
	if authorizationHeader == "" {
		//logrus.Warn("Unauthenticated request as no authorization header was provided")
		det := helper.NewDetail("repository", req.Repo, req.Method)
		unauthorized(w, realm, det, invalidRequest, "missing authorization header")
		return fmt.Errorf("missing authorization header")
	}
	bearerToken := strings.Split(authorizationHeader, " ")
	if len(bearerToken) != 2 {
		repo := req.Repo
		if req.Repo == "" {
			repo = "repo"
		}
		det := helper.NewDetail("repository", repo, "pull")
		w.WriteHeader(http.StatusUnauthorized)
		utils.Debug(l, span, realFrmt, realm)
		unauthorized(w, realm, det, invalidRequest, "invalid authorization header: %q", authorizationHeader)
		return fmt.Errorf("valid authorization header")
	}
	accessTokenString := bearerToken[1]
	at, err := h.mag.UserManagement.User.GetAccessToken(accessTokenString)
	if err != nil {
		l.Infof("error getting access token: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return FailGetToken
	}
	if at == nil {
		l.Error(uknAccTok)
		det := helper.NewDetail("repository", req.Repo, req.Method)
		w.WriteHeader(http.StatusUnauthorized)
		logrus.Infof(realFrmt, realm)
		unauthorized(w, realm, det, invalidToken, uknAccTok)
		return FailGetToken
	}
	var username, profile, password string
	// The access token is the base64 encoded username and password we need to check
	decoded := at.GetUser()
	parts := strings.Split(string(decoded), ":")
	if len(parts) != 2 {
		l.Error(uknAccTok)
		det := helper.NewDetail("repository", req.Repo, req.Method)
		w.WriteHeader(http.StatusUnauthorized)
		utils.Debug(l, span, realFrmt, realm)
		unauthorized(w, realm, det, invalidToken, uknAccTok)
		return FailGetToken
	}
	password = parts[1]
	m, err := utils.ReSubMatchMap(userReg, parts[0])
	if err != nil {
		utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "pull", err, "INTERNAL_ERROR", "failed to match regex")
		return
	}
	if _, ok := m["user"]; !ok {
		msg := fmt.Sprintf("username needs to prefix with profile (defaults to %s): '<user>/<profile>'", defaultProfile)
		logrus.Error(msg)
		w.WriteHeader(http.StatusBadRequest)
		return fmt.Errorf("%s", msg)
	}
	username = m["user"]
	if _, ok := m["profile"]; !ok {
		utils.Debug(l, span, "no profile given when matching '%s' against '%s', default to '%s'", userReg, parts[0], defaultProfile)
		profile = defaultProfile
	} else {
		profile = m["profile"]
		if profile == "" {
			profile = defaultProfile
		}
		utils.Debug(l, span, "set profile to default '%s'", defaultProfile)
	}
	utils.Debug(l, span, "profile:'%s' / user:'%s", profile, username)
	code, uRole, pl, err := h.mag.UserManagement.LoginUserScope(ctx, username, password, req)
	if err != nil {
		err = errors.Wrap(err, "error logging in user")
		logrus.Error(err)
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	switch code {
	case http.StatusOK:
		// Return the account name to the context
		gCtx.Set(r, "accountName", at.Token)
		gCtx.Set(r, "userName", username)
		gCtx.Set(r, "userRole", uRole)
		gCtx.Set(r, "profileName", profile)
		utils.Debug(l, span, "set profileName:'%s'", profile)
		utils.Debug(l, span, "set userRole:'%s'", uRole)
	default:
		helper.WriteOCIErrror(w, http.StatusUnauthorized, pl)
		return fmt.Errorf("error logging in")
	}
	return nil
}

func (h Handlers) AuthMiddlewareFunc(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameAuth,
		"func":    "AuthMiddlewareFunc",
	})
	defer span.End()
	_ = ctx
	err := h.AuthMiddlewareHandler(w, r)
	if err != nil {
		return
	}
	uName := gCtx.Get(r, "userName")

	uRole := gCtx.Get(r, "userRole")
	debug, ok := debugUser[uName.(string)]
	if ok {
		gCtx.Set(r, "debug", debug)
	} else {
		gCtx.Set(r, "debug", 0)
	}
	utils.Debug(l, span, "set debug for user '%s' (role: %s) to level '%d'", uName, uRole, debug)

}

func (h Handlers) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.AuthMiddlewareFunc(w, r)
		next.ServeHTTP(w, r)
	})
}

// unauthorized returns a request to the user for auth - e.g., "get a token from this endpoint"
func unauthorized(w http.ResponseWriter, realm string, det helper.Detail, err oauthError, format string, v ...interface{}) {
	l := logrus.WithFields(logrus.Fields{
		"package": "middleware",
		"func":    "unauthorized",
	})
	_ = l
	description := fmt.Sprintf(format, v...)
	authUrl := realm + "/auth"
	authenticateHeader := fmt.Sprintf("Bearer realm=%q,error=%q,error_description=%q", authUrl, err, description)
	w.Header().Add("Content-Type", "application/json")
	l.Infof("authenticateHeader: %s", authenticateHeader)
	w.Header().Add("WWW-Authenticate", authenticateHeader)
	//w.Header().Del("Vary")
	//w.Header().Add("Docker-Distribution-Api-Version", "registry/2.0")
	///{"errors":[{"code":"UNAUTHORIZED","message":"access to the requested resource is not authorized","detail":[{"Type":"repository","Name":"samalba/my-app","Action":"pull"},{"Type":"repository","Name":"samalba/my-app","Action":"push"}]}]}
	msg := "authentication required"
	e := helper.NewErrPayload("UNAUTHORIZED", msg, det)
	helper.WriteOCIErrror(w, http.StatusUnauthorized, e)
}
