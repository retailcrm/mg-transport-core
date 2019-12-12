package core

import (
	"bytes"
	// nolint:gosec
	"crypto/sha1"
	"encoding/base64"
	"io"
	"io/ioutil"
	"math/rand"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
)

// CSRFErrorReason is a error reason type
type CSRFErrorReason uint8

// CSRFTokenGetter func type
type CSRFTokenGetter func(*gin.Context) string

// CSRFAbortFunc is a callback which
type CSRFAbortFunc func(*gin.Context, CSRFErrorReason)

const (
	// CSRFErrorNoTokenInSession will be returned if token is not present in session
	CSRFErrorNoTokenInSession CSRFErrorReason = iota

	// CSRFErrorCannotStoreTokenInSession will be returned if middleware cannot store token in session
	CSRFErrorCannotStoreTokenInSession

	// CSRFErrorIncorrectTokenType will be returned if data type of token in session is not string
	CSRFErrorIncorrectTokenType

	// CSRFErrorEmptyToken will be returned if token in session is empty
	CSRFErrorEmptyToken

	// CSRFErrorTokenMismatch will be returned in case of invalid token
	CSRFErrorTokenMismatch
)

// DefaultCSRFTokenGetter default getter
var DefaultCSRFTokenGetter = func(c *gin.Context) string {
	r := c.Request

	if t := r.URL.Query().Get("csrf_token"); len(t) > 0 {
		return t
	} else if t := r.Header.Get("X-CSRF-Token"); len(t) > 0 {
		return t
	} else if t := r.Header.Get("X-XSRF-Token"); len(t) > 0 {
		return t
	} else if c.Request.Body != nil {
		data, _ := ioutil.ReadAll(c.Request.Body)
		c.Request.Body = ioutil.NopCloser(bytes.NewReader(data))
		t := r.FormValue("csrf_token")
		c.Request.Body = ioutil.NopCloser(bytes.NewReader(data))

		if len(t) > 0 {
			return t
		}
	}

	return ""
}

// DefaultIgnoredMethods ignored methods for CSRF verifier middleware
var DefaultIgnoredMethods = []string{"GET", "HEAD", "OPTIONS"}

// CSRF struct. Provides CSRF token verification.
type CSRF struct {
	salt            string
	secret          string
	sessionName     string
	abortFunc       CSRFAbortFunc
	csrfTokenGetter CSRFTokenGetter
	store           sessions.Store
}

// NewCSRF creates CSRF struct with specified configuration and session store.
// GenerateCSRFMiddleware and VerifyCSRFMiddleware returns CSRF middlewares.
// Salt must be different every time (pass empty salt to use random), secret must be provided, sessionName is optional - pass empty to use default,
// store will be used to store sessions, abortFunc will be called to return error if token is invalid, csrfTokenGetter will be used to obtain token.
// Usage (with random salt):
// 		core.NewCSRF("", "super secret", "csrf_session", store, func (c *gin.Context, reason core.CSRFErrorReason) {
// 			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid CSRF token"})
// 		}, core.DefaultCSRFTokenGetter)
// Note for csrfTokenGetter: if you want to read token from request body (for example, from form field) - don't forget to restore Body data!
// Body in http.Request is io.ReadCloser instance. Reading CSRF token from form like that:
// 		if t := r.FormValue("csrf_token"); len(t) > 0 {
// 			return t
// 		}
// will close body - and all next middlewares won't be able to read body at all!
// Use DefaultCSRFTokenGetter as example to implement your own token getter.
// CSRFErrorReason will be passed to abortFunc and can be used for better error messages.
func NewCSRF(salt, secret, sessionName string, store sessions.Store, abortFunc CSRFAbortFunc, csrfTokenGetter CSRFTokenGetter) *CSRF {
	if store == nil {
		panic("store must not be nil")
	}

	if secret == "" {
		panic("at least secret must be provided")
	}

	csrf := &CSRF{
		store:           store,
		secret:          secret,
		abortFunc:       abortFunc,
		csrfTokenGetter: csrfTokenGetter,
	}

	if salt == "" {
		salt = csrf.generateSalt()
	}

	if sessionName == "" {
		sessionName = "csrf_token_session"
	}

	csrf.salt = salt
	csrf.sessionName = sessionName

	return csrf
}

// strInSlice checks whether string exists in slice
func (x *CSRF) strInSlice(slice []string, v string) bool {
	exists := false

	for _, i := range slice {
		if i == v {
			exists = true
			break
		}
	}

	return exists
}

// generateCSRFToken generates new CSRF token
func (x *CSRF) generateCSRFToken() string {
	// nolint:gosec
	h := sha1.New()
	// Fallback to less secure method - token must be always filled even if we cannot properly generate it
	if _, err := io.WriteString(h, x.salt+"#"+x.secret); err != nil {
		return base64.URLEncoding.EncodeToString([]byte(time.Now().String()))
	}
	hash := base64.URLEncoding.EncodeToString(h.Sum(nil))

	return hash
}

// generateSalt generates salt from random bytes. If it fails to generate cryptographically
// secure salt - it will generate pseudo-random, weaker salt.
// It will be used automatically if no salt provided.
// Default secure salt length: 8 bytes.
// Default pseudo-random salt length: 64 bytes.
func (x *CSRF) generateSalt() string {
	salt := securecookie.GenerateRandomKey(8)

	if salt == nil {
		return x.pseudoRandomString(64)
	}

	return string(salt)
}

// pseudoRandomString generates pseudo-random string with specified length
func (x *CSRF) pseudoRandomString(length int) string {
	rand.Seed(time.Now().UnixNano())
	data := make([]byte, length)

	for i := 0; i < length; i++ {
		data[i] = byte(65 + rand.Intn(90-65))
	}

	return string(data)
}

// CSRFFromContext returns csrf token or random token. It shouldn't return empty string because it will make csrf protection useless.
// e.g. any request without token will work fine, which is inacceptable.
func (x *CSRF) CSRFFromContext(c *gin.Context) string {
	if i, ok := c.Get("csrf_token"); ok {
		if token, ok := i.(string); ok {
			return token
		}
	}

	return x.generateCSRFToken()
}

// GenerateCSRFMiddleware returns gin.HandlerFunc which will generate CSRF token
// Usage:
//      engine := gin.New()
//      csrf := NewCSRF("salt", "secret", "not_found", "incorrect", localizer)
//      engine.Use(csrf.GenerateCSRFMiddleware())
func (x *CSRF) GenerateCSRFMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		session, _ := x.store.Get(c.Request, x.sessionName)

		if i, ok := session.Values["csrf_token"]; ok {
			if i, ok := i.(string); !ok || i == "" {
				if x.fillToken(session, c) != nil {
					x.abortFunc(c, CSRFErrorCannotStoreTokenInSession)
					c.Abort()
					return
				}
			}
		} else {
			if x.fillToken(session, c) != nil {
				x.abortFunc(c, CSRFErrorCannotStoreTokenInSession)
				c.Abort()
				return
			}
		}
	}
}

// fillToken stores token in session and context
func (x *CSRF) fillToken(s *sessions.Session, c *gin.Context) error {
	s.Values["csrf_token"] = x.generateCSRFToken()
	c.Set("csrf_token", s.Values["csrf_token"])
	return s.Save(c.Request, c.Writer)
}

// VerifyCSRFMiddleware verifies CSRF token
// Usage:
// 		engine := gin.New()
// 		engine.Use(csrf.VerifyCSRFMiddleware())
func (x *CSRF) VerifyCSRFMiddleware(ignoredMethods []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if x.strInSlice(ignoredMethods, c.Request.Method) {
			return
		}

		var token string
		session, _ := x.store.Get(c.Request, x.sessionName)

		if i, ok := session.Values["csrf_token"]; ok {
			var v string
			if v, ok = i.(string); !ok || v == "" {
				if !ok {
					x.abortFunc(c, CSRFErrorIncorrectTokenType)
				} else if v == "" {
					x.abortFunc(c, CSRFErrorEmptyToken)
				}

				c.Abort()
				return
			}

			token = v
		} else {
			x.abortFunc(c, CSRFErrorNoTokenInSession)
			c.Abort()
			return
		}

		if x.csrfTokenGetter(c) != token {
			x.abortFunc(c, CSRFErrorTokenMismatch)
			c.Abort()
			return
		}
	}
}

// GetCSRFErrorMessage returns generic error message for CSRFErrorReason in English (useful for logs)
func GetCSRFErrorMessage(r CSRFErrorReason) string {
	switch r {
	case CSRFErrorNoTokenInSession:
		return "token is not present in session"
	case CSRFErrorCannotStoreTokenInSession:
		return "cannot store token in session"
	case CSRFErrorIncorrectTokenType:
		return "incorrect token type"
	case CSRFErrorEmptyToken:
		return "empty token present in session"
	case CSRFErrorTokenMismatch:
		return "token mismatch"
	default:
		return "unknown error"
	}
}
