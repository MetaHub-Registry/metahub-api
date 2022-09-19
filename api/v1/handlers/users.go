package handlers

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-api/internal/data"
)

func (h Handlers) Login(w http.ResponseWriter, r *http.Request) {
	logrus.Info("Login")
	type credentials struct {
		UserName string `json:"username"`
		Password string `json:"password"`
	}
	var creds credentials
	var payload jsonResponse

	err := h.readJSON(w, r, &creds)
	if err != nil {
		payload.Error = true
		payload.Message = "invalid json supplied, or json missing entirely"
		_ = h.writeJSON(w, http.StatusBadRequest, payload)
	}

	// look up the user by email
	user, err := h.models.User.GetByUsername(creds.UserName)
	if err != nil {
		h.errorJSON(w, errors.New("invalid username/password"))
		return
	}

	// validate the user's password
	validPassword, err := user.PasswordMatches(creds.Password)
	if err != nil || !validPassword {
		h.errorJSON(w, errors.New("invalid username/password"))
		return
	}
	// make sure the user is active
	if user.Active == 0 {
		h.errorJSON(w, errors.New("user is not active"))
		return
	}

	// we have a valid user, so generate a token
	token, err := h.models.Token.GenerateToken(user.ID, 24*time.Hour)
	if err != nil {
		h.errorJSON(w, err)
		return
	}

	// save it to the database
	err = h.models.Token.Insert(*token, *user)
	if err != nil {
		h.errorJSON(w, err)
		return
	}

	// send back a response
	payload = jsonResponse{
		Error:   false,
		Message: "logged in",
		Data: envelope{
			"token": token,
			"user":  user,
		},
	}

	err = h.writeJSON(w, http.StatusOK, payload)
	if err != nil {
		log.Fatal(err)
	}
}

func (h Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	var requestPayload struct {
		Token string `json:"token"`
	}

	err := h.readJSON(w, r, &requestPayload)
	if err != nil {
		h.errorJSON(w, errors.New("invalid json"))
		return
	}

	err = h.models.Token.DeleteByToken(requestPayload.Token)
	if err != nil {
		h.errorJSON(w, errors.New("invalid json"))
		return
	}

	payload := jsonResponse{
		Error:   false,
		Message: "logged out",
	}

	_ = h.writeJSON(w, http.StatusOK, payload)
}

func (h Handlers) Signup(w http.ResponseWriter, r *http.Request) {
	logrus.Debug("Signup")
	type signupUser struct {
		UserName string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	// read the request body
	var newUser signupUser
	err := h.readJSON(w, r, &newUser)
	if err != nil {
		h.errorJSON(w, errors.New("invalid json"))
		return
	}
	u := data.User{
		UserName:  newUser.UserName,
		Email:     newUser.Email,
		Password:  newUser.Password,
		Active:    0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// insert user into DB
	_, err = h.models.User.Insert(u)
	if err != nil {
		h.errorJSON(w, err)
		return
	}

	// send activation email

}
