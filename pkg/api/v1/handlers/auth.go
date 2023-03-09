package handlers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/gorilla/context"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/helper"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type oauthError string

var (
	invalidRequest oauthError = "invalid_request"
	invalidToken   oauthError = "invalid_token"
	userReg                   = regexp.MustCompile(`(?P<user>[\w\-\.\_]+)(\/(?P<profile>[\/\w\-\.\_#]+))?`)
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
	_, span := tracer.Start(ctx, "AuthHandler")
	defer span.End()
	acct := r.URL.Query().Get("account")
	at, user, err := getAccessTokenFromBearerToken(r)
	if err != nil {
		err = errors.Wrap(err, "error getting access token from bearer token")
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.message", fmt.Sprintf("user:'%s' acct:'%s'", user, acct)),
	))
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
	ctx := r.Context()
	_, span := tracer.Start(ctx, "AuthMiddlewareHandler")
	defer span.End()
	req := types.NewRequest(ctx, r)
	span.SetAttributes(
		attribute.Key("req.Digest").String(req.Digest),
		attribute.Key("req.Host").String(req.Host),
		attribute.Key("req.Repo").String(req.Repo),
		attribute.Key("req.Tag").String(req.Tag),
		attribute.Key("req.Profile").String(req.Profile),
	)
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
		logrus.Infof("Realm: %s", realm)
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
		logrus.Infof("Realm: %s", realm)
		unauthorized(w, realm, det, invalidToken, "unknown access token")
		return fmt.Errorf("error getting token")
	}
	var username, profile, password string
	// The access token is the base64 encoded username and password we need to check
	decoded := at.GetUser()
	parts := strings.Split(string(decoded), ":")
	if len(parts) != 2 {
		logrus.Error("unknown access token")
		det := helper.NewDetailFromReq(req)
		w.WriteHeader(http.StatusUnauthorized)
		logrus.Infof("Realm: %s", realm)
		unauthorized(w, realm, det, invalidToken, "unknown access token")
		return fmt.Errorf("error getting token")
	}
	password = parts[1]
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.user", parts[0]),
	))
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.message", fmt.Sprintf("matching '%s' against '%s'", userReg, parts[0])),
	))
	m := utils.ReSubMatchMap(userReg, parts[0])
	if _, ok := m["user"]; !ok {
		msg := "username needs to prefix with profile (defaults to generic): '<user>/<profile>'"
		logrus.Error(msg)
		w.WriteHeader(http.StatusBadRequest)
		return fmt.Errorf(msg)
	}
	username = m["user"]
	if _, ok := m["profile"]; !ok {
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "debug"),
			attribute.String("log.message", fmt.Sprintf("no profile given when matching '%s' against '%s', default to 'generic'", userReg, parts[0])),
		))
		profile = "generic"
	} else {
		profile = m["profile"]
	}
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.message", fmt.Sprintf("profile:'%s'", profile)),
	))
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
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "debug"),
			attribute.String("log.message", fmt.Sprintf("set profileName:'%s'", profile)),
		))
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
	l.Infof("authenticateHeader: %s", authenticateHeader)
	w.Header().Add("WWW-Authenticate", authenticateHeader)
	//w.Header().Del("Vary")
	//w.Header().Add("Docker-Distribution-Api-Version", "registry/2.0")
	///{"errors":[{"code":"UNAUTHORIZED","message":"access to the requested resource is not authorized","detail":[{"Type":"repository","Name":"samalba/my-app","Action":"pull"},{"Type":"repository","Name":"samalba/my-app","Action":"push"}]}]}
	msg := "authentication required"
	e := helper.NewErrPayload("UNAUTHORIZED", msg, det)
	helper.WriteOCIErrror(w, http.StatusUnauthorized, e)
}
