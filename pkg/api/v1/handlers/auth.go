package handlers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/context"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/helper"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

type oauthError string

var (
	invalidRequest oauthError = "invalid_request"
	invalidToken   oauthError = "invalid_token"
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
	acct := r.URL.Query().Get("account")
	at, user, err := getAccessTokenFromBearerToken(r)
	if err != nil {
		err = errors.Wrap(err, "error getting access token from bearer token")
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	if acct == "" && user == "" {
		at.Token = "REDACTED-IN-HISTORY"
	}
	// Prepare the body of the response
	responseBodyBytes := new(bytes.Buffer)
	w.Header().Add("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(responseBodyBytes).Encode(at)
	data := responseBodyBytes.Bytes()
	w.Write(data)
}

func (h Handlers) AuthMiddlewareHandler(w http.ResponseWriter, r *http.Request) (err error) {
	req := types.NewRequest(r)
	var realm string
	if r.Host == "metahub-registry.org" {
		realm = "https://metahub-registry.org"
	} else {
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
		det := helper.NewDetailFromReq(req)
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
		unauthorized(w, realm, det, invalidRequest, "invalid authorization header: %q", authorizationHeader)
		return fmt.Errorf("valid authorization header")
	}
	accessTokenString := bearerToken[1]

	at, err := h.mag.UserManagement.User.GetAccessToken(accessTokenString)
	if err != nil {
		logrus.Infof("error getting access token: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return fmt.Errorf("error getting token")
	}
	if at == nil {
		logrus.Error("unknown access token")
		det := helper.NewDetailFromReq(req)
		w.WriteHeader(http.StatusUnauthorized)
		unauthorized(w, realm, det, invalidToken, "unknown access token")
		return fmt.Errorf("error getting token")
	}
	var username, profile, password string
	// The access token is the base64 encoded username and password we need to check
	decoded := at.GetUser()
	// Ensure the login is correct
	parts := strings.Split(decoded, ":")
	switch len(parts) {
	case 1:
		username = "generic"
		profile = "generic"
	case 2:
		password = parts[1]
		profSplit := strings.Split(parts[0], "/")
		switch len(profSplit) {
		case 1:
			username = parts[0]
			profile = "generic"
		case 2:
			username = profSplit[0]
			profile = profSplit[1]
		default:
			msg := "username needs to prefix with profile (defaults to generic): '<user>/<profile>'"
			log.Print(msg)
			w.WriteHeader(http.StatusBadRequest)
			return fmt.Errorf(msg)
		}

	default:
		msg := "username needs to prefix with profile (defaults to generic): '<user>/<profile>'"
		log.Print(msg)
		w.WriteHeader(http.StatusBadRequest)
		return fmt.Errorf(msg)
	}
	code, pl, err := h.mag.UserManagement.LoginUserScope(username, password, req)
	if err != nil {
		err = errors.Wrap(err, "error logging in user")
		logrus.Error(err)
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	switch code {
	case http.StatusOK:
		// Return the account name to the context
		context.Set(r, "accountName", at.Token)
		context.Set(r, "userName", username)
		context.Set(r, "profileName", profile)
	default:
		helper.WriteOCIErrror(w, http.StatusUnauthorized, pl)
		return fmt.Errorf("error logging in")
	}
	return nil
}

func (h Handlers) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := h.AuthMiddlewareHandler(w, r)
		if err != nil {
			return
		}
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
	//l.Infof("authenticateHeader: %s", authenticateHeader)
	w.Header().Add("WWW-Authenticate", authenticateHeader)
	//w.Header().Del("Vary")
	//w.Header().Add("Docker-Distribution-Api-Version", "registry/2.0")
	///{"errors":[{"code":"UNAUTHORIZED","message":"access to the requested resource is not authorized","detail":[{"Type":"repository","Name":"samalba/my-app","Action":"pull"},{"Type":"repository","Name":"samalba/my-app","Action":"push"}]}]}
	msg := "authentication required"
	e := helper.NewErrPayload("UNAUTHORIZED", msg, det)
	helper.WriteOCIErrror(w, http.StatusUnauthorized, e)
}
