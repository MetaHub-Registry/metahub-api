package sqlite

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"net/http"
	"strings"
	"time"
)

// Token is the data structure for any token in the database. Note that
// we do not send the TokenHash (a slice of bytes) in any exported JSON.
type Token struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	Email     string    `json:"email"`
	Token     string    `json:"token"`
	TokenHash []byte    `json:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Expiry    time.Time `json:"expiry"`
}

// GetByToken takes a plain text token string, and looks up the full token from
// the database. It returns a pointer to the Token model.
func (t *Token) GetByToken(plainText string) (*Token, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := `select id, user_id, email, token, token_hash, created_at, updated_at, expiry
			from tokens where token = $1`

	var token Token

	row := db.QueryRowContext(ctx, query, plainText)
	err := row.Scan(
		&token.ID,
		&token.UserID,
		&token.Email,
		&token.Token,
		&token.TokenHash,
		&token.CreatedAt,
		&token.UpdatedAt,
		&token.Expiry,
	)

	if err != nil {
		return nil, err
	}

	return &token, nil
}

// GetUserForToken takes a token parameter, and uses the UserID field from that parameter
// to look a user up by id. It returns a pointer to the user model.
func (t *Token) GetUserForToken(token Token) (*User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := `select id, email, first_name, last_name, password, user_active, created_at, updated_at from users where id = $1`

	var user User
	row := db.QueryRowContext(ctx, query, token.UserID)

	err := row.Scan(
		&user.ID,
		&user.Email,
		&user.FirstName,
		&user.LastName,
		&user.Password,
		&user.Active,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &user, nil
}

// GenerateToken generates a secure token of exactly 26 characters in length and returns it
func (t *Token) GenerateToken(userID int, ttl time.Duration) (*Token, error) {
	token := &Token{
		UserID: userID,
		Expiry: time.Now().Add(ttl),
	}

	randomBytes := make([]byte, 16)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return nil, err
	}

	token.Token = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(randomBytes)
	hash := sha256.Sum256([]byte(token.Token))
	token.TokenHash = hash[:]

	return token, nil
}

// AuthenticateToken takes the full http request, extracts the authorization header,
// takes the plain text token from that header and looks up the associated token entry
// in the database, and then finds the user associated with that token. If the token
// is valid and a user is found, the user is returned; otherwise, it returns an error.
func (t *Token) AuthenticateToken(r *http.Request) (*User, error) {
	// get the authorization header
	authorizationHeader := r.Header.Get("Authorization")
	if authorizationHeader == "" {
		return nil, errors.New("no authorization header received")
	}

	// get the plain text token from the header
	headerParts := strings.Split(authorizationHeader, " ")
	if len(headerParts) != 2 || headerParts[0] != "Bearer" {
		return nil, errors.New("no valid authorization header received")
	}

	token := headerParts[1]

	// make sure the token is of the correct length
	if len(token) != 26 {
		return nil, errors.New("token wrong size")
	}

	// get the token from the database, using the plain text token to find it
	tkn, err := t.GetByToken(token)
	if err != nil {
		return nil, errors.New("no matching token found")
	}

	// make sure the token has not expired
	if tkn.Expiry.Before(time.Now()) {
		return nil, errors.New("expired token")
	}

	// get the user associated with the token
	user, err := t.GetUserForToken(*tkn)
	if err != nil {
		return nil, errors.New("no matching user found")
	}
	// make sure user is active
	if user.Active == 0 {
		return nil, errors.New("user is not active")
	}

	return user, nil
}

// Insert a token into the database
func (t *Token) Insert(token Token, u User) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	// delete any existing tokens
	stmt := `delete from tokens where user_id = $1`
	_, err := db.ExecContext(ctx, stmt, token.UserID)
	if err != nil {
		return err
	}

	// we assign the email value, just to be safe, in case it was
	// not done in the handler that calls this function
	token.Email = u.Email

	// insert the new token
	stmt = `insert into tokens (user_id, email, token, token_hash, created_at, updated_at, expiry)
		values ($1, $2, $3, $4, $5, $6, $7)`

	_, err = db.ExecContext(ctx, stmt,
		token.UserID,
		token.Email,
		token.Token,
		token.TokenHash,
		time.Now(),
		time.Now(),
		token.Expiry,
	)
	if err != nil {
		return err
	}

	return nil
}

// DeleteByToken deletes a token, by plain text token
func (t *Token) DeleteByToken(plainText string) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	stmt := `delete from tokens where token = $1`

	_, err := db.ExecContext(ctx, stmt, plainText)
	if err != nil {
		return err
	}

	return nil
}

// ValidToken makes certain that a given token is valid; in order to be valid,
// the token must exist in the database, the associated user must exist in the database,
// and the token must not have expired.
func (t *Token) ValidToken(plainText string) (bool, error) {
	token, err := t.GetByToken(plainText)
	if err != nil {
		return false, errors.New("no matching token found")
	}

	_, err = t.GetUserForToken(*token)
	if err != nil {
		return false, errors.New("no matching user found")
	}

	if token.Expiry.Before(time.Now()) {
		return false, errors.New("expired token")
	}

	return true, nil
}

func (t *Token) DeleteTokensForUser(id int) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	// delete any existing tokens
	stmt := `DELETE from tokens where user_id = $1`
	_, err := db.ExecContext(ctx, stmt, id)
	if err != nil {
		return err
	}
	return nil
}
