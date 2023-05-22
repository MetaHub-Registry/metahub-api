package handlers

import (
	"encoding/json"
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
		ClientID:     "Iv1.dd76eb11191acb93",
		ClientSecret: "REDACTED-IN-HISTORY",
		// select level of access you want https://developer.github.com/v3/oauth/#scopes
		Scopes:   []string{"user"},
		Endpoint: githuboauth.Endpoint,
	}
	// random string for oauth2 API calls to protect against CSRF
	oauthStateString = "thisshouldberandom"
)

func (h Handlers) GitHubLogin(w http.ResponseWriter, r *http.Request) {
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
	state := r.FormValue("state")
	if state != oauthStateString {
		fmt.Printf("invalid oauth state, expected '%s', got '%s'\n", oauthStateString, state)
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

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
	usr := types.UserFromGitHubUser(user)
	if ok, e := h.mag.UserManagement.UserNameExists(ctx, usr.Username); e != nil {
		utils.ErrorHandlerPlainWWW(w, l, span, e, "INTERNAL ERROR", "failed to check if user exists")
		return
	} else if !ok {
		if _, e := h.mag.UserManagement.InsertUser(ctx, usr); e != nil {
			utils.ErrorHandlerPlainWWW(w, l, span, e, "INTERNAL ERROR", "failed to insert user")
			return
		}
	}
	cookie := http.Cookie{
		Name:     "loginName",
		Value:    usr.Username,
		Path:     "/",
		MaxAge:   3600,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
	}
	http.SetCookie(w, &cookie)
	cookie = http.Cookie{
		Name:     "userId",
		Value:    usr.GithubID,
		Path:     "/",
		MaxAge:   3600,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
	}
	http.SetCookie(w, &cookie)

	fmt.Printf("Logged in as GitHub user: %s. forwording to /\n", *user.Login)
	red := "http://localhost:8081/profile"
	http.Redirect(w, r, red, http.StatusTemporaryRedirect)
}

func tokenToJSON(token *oauth2.Token) (string, error) {
	if d, err := json.Marshal(token); err != nil {
		return "", err
	} else {
		return string(d), nil
	}
}

func tokenFromJSON(jsonStr string) (*oauth2.Token, error) {
	var token oauth2.Token
	if err := json.Unmarshal([]byte(jsonStr), &token); err != nil {
		return nil, err
	}
	return &token, nil
}
