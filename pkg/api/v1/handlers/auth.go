package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/context"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-api/pkg/lib/account"
)

func (h Handlers) AuthHandler(w http.ResponseWriter, r *http.Request) {
	acct := r.URL.Query().Get("account")
	_ = acct
	authorizationHeader := r.Header.Get("authorization")
	var at *account.AccessToken
	if acct == "" {
		at = &account.AccessToken{
			// TODO: this token should represent the user account 'generic'
			Token: "REDACTED-IN-HISTORY",
		}
	} else {
		if authorizationHeader == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		bearerToken := strings.Split(authorizationHeader, " ")
		if len(bearerToken) != 2 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		accessTokenString := bearerToken[1]
		var err error
		at, err = h.mag.SQlite.User.GetAccessToken(accessTokenString)
		if err != nil || at == nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}
	// Prepare the body of the response
	responseBodyBytes := new(bytes.Buffer)
	json.NewEncoder(responseBodyBytes).Encode(at)
	data := responseBodyBytes.Bytes()
	w.Write(data)
}

func (h Handlers) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		//dumpedBody, _ := httputil.DumpRequest(r, true)
		//fmt.Printf("%s", dumpedBody)
		authorizationHeader := r.Header.Get("authorization")
		if authorizationHeader == "" {
			logrus.Warn("Unauthenticated request")
			unauthorized(w, realm, invalidRequest, "missing authorization header")
			return

		}
		bearerToken := strings.Split(authorizationHeader, " ")
		if len(bearerToken) != 2 {
			unauthorized(w, realm, invalidRequest, "invalid authorization header: %q", authorizationHeader)
			return
		}
		accessTokenString := bearerToken[1]

		at, err := h.mag.SQlite.User.GetAccessToken(accessTokenString)
		if err != nil {
			logrus.Infof("error getting access token: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if at == nil {
			logrus.Error("unknown access token")
			unauthorized(w, realm, invalidToken, "unknown access token")
			return
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
				log.Printf("username needs to prefix with profile (defaults to generic): '<user>/<profile>'")
				w.WriteHeader(http.StatusBadRequest)
				return
			}

		default:
			log.Printf("username needs to prefix with profile (defaults to generic): '<user>/<profile>'")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_ = password
		_ = username
		_ = profile

		//err = storage.Login(username, profile, password)
		err = h.mag.SQlite.LoginUser(username, profile, password)
		if err != nil {
			logrus.Error(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Return the account name to the context
		context.Set(r, "accountName", at.Token)
		context.Set(r, "userName", username)
		context.Set(r, "profileName", profile)
		next.ServeHTTP(w, r)
	})
}

type oauthError string

var (
	invalidRequest oauthError = "invalid_request"
	invalidToken   oauthError = "invalid_token"
)

// unauthorized returns a request to the user for auth - e.g., "get a token from this endpoint"
func unauthorized(w http.ResponseWriter, realm string, err oauthError, format string, v ...interface{}) {
	l := logrus.WithFields(logrus.Fields{
		"package": "middleware",
		"func":    "unauthorized",
	})
	_ = l
	description := fmt.Sprintf(format, v...)
	authUrl := realm + "/auth"
	authenticateHeader := fmt.Sprintf("Bearer realm=%q,error=%q,error_description=%q", authUrl, err, description)

	//l.Infof("authenticateHeader: %s", authenticateHeader)
	w.Header().Add("WWW-Authenticate", authenticateHeader)
	w.Header().Del("Vary")
	w.Header().Add("Docker-Distribution-Api-Version", "registry/2.0")
	///{"errors":[{"code":"UNAUTHORIZED","message":"access to the requested resource is not authorized","detail":[{"Type":"repository","Name":"samalba/my-app","Action":"pull"},{"Type":"repository","Name":"samalba/my-app","Action":"push"}]}]}

	w.WriteHeader(http.StatusUnauthorized)
}
