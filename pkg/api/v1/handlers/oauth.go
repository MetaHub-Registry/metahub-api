package handlers

import (
	"fmt"
	"net/http"

	"github.com/google/go-github/github"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"golang.org/x/oauth2"
	githuboauth "golang.org/x/oauth2/github"
)

const (
	pkgNameOauth = "hanlders/oauth.go"
)

var (
	// You must register the app at https://github.com/settings/applications
	// Set callback to http://127.0.0.1:7000/github_oauth_cb
	// Set ClientId and ClientSecret to
	oauthConf = &oauth2.Config{
		ClientID: "",
	}
)

func (h Handlers) GitHubLogout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameOauth,
		"func":    "GitHubLogout",
		"method":  r.Method,
	})
	defer span.End()
	uname, err := h.getUsername(ctx, r)
	if err != nil {
		utils.ErrorHandlerPlainWWW(w, l, span, err, "INTERNAL_ERROR", "failed to get username")
		return
	}
	sqlUser, err := h.mag.UserManagement.GetByUsername(ctx, uname)
	if err != nil {
		utils.ErrorHandlerPlainWWW(w, l, span, err, "INTERNAL_ERROR", "failed to get user '%s'", uname)
		return
	}
	if e := h.mag.UserManagement.SetLoggedOut(ctx, sqlUser.ID); e != nil {
		utils.ErrorHandlerPlainWWW(w, l, span, e, "INTERNAL_ERROR", "failed to set user '%s' logged out", uname)
		return
	}
	utils.Info(l, span, "Logged out user: '%s'. forwording to '%s'", uname, h.mag.Config.RedirectDoc)
}

func (h Handlers) GitHubLogin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameOauth,
		"func":    "GitHubLogin",
		"method":  r.Method,
	})

	defer span.End()
	_ = ctx
	if !h.mag.Config.OAuth.Enabled {
		utils.NewErrorPlainWWW(w, l, span, "FORBIDDEN", "OAuth not enabled")
	} else if oauthConf.ClientID == "" {
		oauthConf = &oauth2.Config{
			ClientID:     h.mag.Config.OAuth.ClientID,
			ClientSecret: h.mag.Config.OAuth.ClientSecret,
			// select level of access you want https://developer.github.com/v3/oauth/#scopes
			Scopes:   []string{"user"},
			Endpoint: githuboauth.Endpoint,
		}
	}
	// random string for oauth2 API calls to protect against CSRF
	oauthStateString := h.mag.Config.OAuth.StateString
	url := oauthConf.AuthCodeURL(oauthStateString, oauth2.AccessTypeOnline)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func (h Handlers) GitHubCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameOauth,
		"func":    "GitHubCallback",
		"method":  r.Method,
	})
	defer span.End()
	oauthStateString := h.mag.Config.OAuth.StateString
	state := r.FormValue("state")
	if state != oauthStateString {
		fmt.Printf("invalid oauth state, expected '%s', got '%s'\n", oauthStateString, state)
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	var usr types.User
	code := r.FormValue("code")
	token, err := oauthConf.Exchange(oauth2.NoContext, code)
	if err != nil {
		fmt.Printf("oauthConf.Exchange() failed with '%s'\n", err)
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}
	oauthClient := oauthConf.Client(oauth2.NoContext, token)
	client := github.NewClient(oauthClient)
	user, _, err := client.Users.Get(ctx, "")
	if err != nil {
		fmt.Printf("client.Users.Get() faled with '%s'\n", err)
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}
	utils.Debug(l, span, "User: %v", user)
	usr = types.UserFromGitHubUser(user)
	if ok, e := h.mag.UserManagement.UserNameExists(ctx, usr.Username); e != nil {
		utils.ErrorHandlerPlainWWW(w, l, span, e, "INTERNAL ERROR", "failed to check if user exists")
		return
	} else if !ok {
		if _, e := h.mag.UserManagement.InsertUser(ctx, usr); e != nil {
			utils.ErrorHandlerPlainWWW(w, l, span, e, "INTERNAL ERROR", "failed to insert user")
			return
		}
	}
	btkn, err := h.CreateToken(ctx, usr.Username, usr.GithubID)
	if err != nil {
		utils.ErrorHandlerPlainWWW(w, l, span, err, "INTERNAL ERROR", "failed to create token")
		return
	}
	sqlUsr, err := h.mag.UserManagement.GetByUsername(ctx, usr.Username)
	if err != nil {
		utils.ErrorHandlerPlainWWW(w, l, span, err, "INTERNAL_ERROR", "failed to get user with name '%s'", usr.Username)
		return
	}
	if e := h.mag.UserManagement.SetLoggedIn(ctx, sqlUsr.ID); e != nil {
		utils.ErrorHandlerPlainWWW(w, l, span, e, "INTERNAL_ERROR", "failed to set user '%s' logged in", usr.Username)
		return
	} else {
		utils.Info(l, span, "Set user '%s' logged in", usr.Username)
	}
	fmt.Printf("Logged in as GitHub user: '%s' (ID:%s). forwording to /profile\n", usr.Username, usr.GithubID)
	red := fmt.Sprintf("%s/profile?cbtoken=%s", h.mag.Config.RedirectDoc, btkn)
	//utils.Debug(l, span, "Token: %s", btkn)
	utils.Debug(l, span, "Redirecting to '%s'", red)
	http.Redirect(w, r, red, http.StatusTemporaryRedirect)
}
