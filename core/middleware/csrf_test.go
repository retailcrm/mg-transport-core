package middleware

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type CSRFTest struct {
	suite.Suite
	csrf *CSRF
}

type requestOptions struct {
	Body    io.Reader
	Headers map[string]string
	Method  string
	URL     string
}

func TestCSRF_DefaultCSRFTokenGetter_Empty(t *testing.T) {
	c := &gin.Context{Request: &http.Request{
		URL: &url.URL{
			RawQuery: "",
		},
		Body: io.NopCloser(bytes.NewReader([]byte(""))),
	}}

	assert.Empty(t, DefaultCSRFTokenGetter(c))
}

func TestCSRF_DefaultCSRFTokenGetter_URL(t *testing.T) {
	c := &gin.Context{Request: &http.Request{
		URL: &url.URL{
			RawQuery: "csrf_token=token",
		},
	}}

	assert.NotEmpty(t, DefaultCSRFTokenGetter(c))
	assert.Equal(t, "token", DefaultCSRFTokenGetter(c))
}

func TestCSRF_DefaultCSRFTokenGetter_Header_CSRF(t *testing.T) {
	header := http.Header{}
	header.Add("X-CSRF-Token", "token")
	c := &gin.Context{Request: &http.Request{
		URL: &url.URL{
			RawQuery: "",
		},
		Header: header,
	}}

	assert.NotEmpty(t, DefaultCSRFTokenGetter(c))
	assert.Equal(t, "token", DefaultCSRFTokenGetter(c))
}

func TestCSRF_DefaultCSRFTokenGetter_Header_XSRC(t *testing.T) {
	header := http.Header{}
	header.Add("X-XSRF-Token", "token")
	c := &gin.Context{Request: &http.Request{
		URL: &url.URL{
			RawQuery: "",
		},
		Header: header,
	}}

	assert.NotEmpty(t, DefaultCSRFTokenGetter(c))
	assert.Equal(t, "token", DefaultCSRFTokenGetter(c))
}

func TestCSRF_DefaultCSRFTokenGetter_Form(t *testing.T) {
	headers := http.Header{}
	headers.Add("Content-Type", "application/x-www-form-urlencoded")
	c := &gin.Context{Request: &http.Request{
		URL: &url.URL{
			RawQuery: "",
		},
		Header: headers,
		Body:   io.NopCloser(bytes.NewReader([]byte(""))),
	}}
	c.Request.PostForm = url.Values{"csrf_token": {"token"}}

	assert.NotEmpty(t, DefaultCSRFTokenGetter(c))
	assert.Equal(t, "token", DefaultCSRFTokenGetter(c))

	_, err := io.ReadAll(c.Request.Body)
	assert.NoError(t, err)
}

func TestCSRF_NewCSRF_NilStore(t *testing.T) {
	defer func() {
		assert.NotNil(t, recover())
	}()

	NewCSRF("salt", "secret", "csrf", nil, func(c *gin.Context, r CSRFErrorReason) {}, DefaultCSRFTokenGetter)
}

func TestCSRF_NewCSRF_EmptySecret(t *testing.T) {
	defer func() {
		assert.NotNil(t, recover())
	}()

	store := sessions.NewCookieStore([]byte("keys"))
	NewCSRF("salt", "", "csrf", store, func(c *gin.Context, r CSRFErrorReason) {}, DefaultCSRFTokenGetter)
}

func TestCSRF_NewCSRF_SaltAndSessionNotEmpty(t *testing.T) {
	store := sessions.NewCookieStore([]byte("keys"))
	csrf := NewCSRF("salt", "secret", "", store, func(c *gin.Context, r CSRFErrorReason) {}, DefaultCSRFTokenGetter)
	assert.NotEmpty(t, csrf.salt)
	assert.NotEmpty(t, csrf.sessionName)
}

func TestCSRF_GetCSRFErrorMessage(t *testing.T) {
	items := map[CSRFErrorReason]string{
		CSRFErrorNoTokenInSession:          "token is not present in session",
		CSRFErrorCannotStoreTokenInSession: "cannot store token in session",
		CSRFErrorIncorrectTokenType:        "incorrect token type",
		CSRFErrorEmptyToken:                "empty token present in session",
		CSRFErrorTokenMismatch:             "token mismatch",
		99:                                 "unknown error",
	}

	for reason, message := range items {
		assert.Equal(t, message, GetCSRFErrorMessage(reason))
	}
}

func TestCSRF_Suite(t *testing.T) {
	suite.Run(t, new(CSRFTest))
}

func (x *CSRFTest) SetupSuite() {
	store := sessions.NewCookieStore([]byte("keys"))
	x.csrf = NewCSRF("salt", "secret", "", store, func(context *gin.Context, r CSRFErrorReason) {
		context.AbortWithStatus(900)
	}, DefaultCSRFTokenGetter)
}

func (x *CSRFTest) NewServer() *gin.Engine {
	gin.SetMode(gin.TestMode)
	g := gin.New()
	g.Use(x.csrf.GenerateCSRFMiddleware(), x.csrf.VerifyCSRFMiddleware(DefaultIgnoredMethods))
	return g
}

func (x *CSRFTest) request(server *gin.Engine, options requestOptions) (*httptest.ResponseRecorder, *http.Request) {
	if options.Method == "" {
		options.Method = "GET"
	}

	w := httptest.NewRecorder()
	req, err := http.NewRequest(options.Method, options.URL, options.Body)

	if options.Headers != nil {
		for key, value := range options.Headers {
			req.Header.Set(key, value)
		}
	}

	server.ServeHTTP(w, req)

	if err != nil {
		panic(err)
	}

	return w, req
}

func (x *CSRFTest) Test_strInSlice() {
	slice := []string{"alpha", "beta", "gamma"}

	assert.False(x.T(), x.csrf.strInSlice(slice, "lambda"))
	assert.True(x.T(), x.csrf.strInSlice(slice, "alpha"))
}

func (x *CSRFTest) Test_generateCSRFToken() {
	assert.NotEmpty(x.T(), x.csrf.generateCSRFToken())
}

func (x *CSRFTest) Test_generateSalt() {
	salt := x.csrf.generateSalt()
	assert.NotEmpty(x.T(), salt)
}

func (x *CSRFTest) Test_pseudoRandomString() {
	assert.Len(x.T(), x.csrf.pseudoRandomString(12), 12)
	assert.Len(x.T(), x.csrf.pseudoRandomString(64), 64)
}

func (x *CSRFTest) Test_CSRFFromContext_NotExist() {
	c := &gin.Context{}
	token := x.csrf.CSRFFromContext(c)

	assert.NotEmpty(x.T(), token)
}

func (x *CSRFTest) Test_CSRFFromContext_NotString() {
	c := &gin.Context{}
	c.Set("csrf_token", struct{}{})
	token := x.csrf.CSRFFromContext(c)

	assert.NotEmpty(x.T(), token)
}

func (x *CSRFTest) Test_CSRFFromContext_Exist() {
	c := &gin.Context{}
	c.Set("csrf_token", "token")
	token := x.csrf.CSRFFromContext(c)

	assert.NotEmpty(x.T(), token)
	assert.Equal(x.T(), "token", token)
}

func (x *CSRFTest) Test_GenerateCSRFMiddleware() {
	assert.NotNil(x.T(), x.csrf.GenerateCSRFMiddleware())
}

func (x *CSRFTest) Test_GenerateCSRFMiddleware_Middleware() {
	x.csrf.store = sessions.NewCookieStore([]byte("secret"))
	g := x.NewServer()
	g.GET("/get", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})
	g.POST("/post", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	get, getReq := x.request(g, requestOptions{
		Method: "GET",
		URL:    "/get",
	})

	session, _ := x.csrf.store.Get(getReq, x.csrf.sessionName)
	post, _ := x.request(g, requestOptions{
		Method: "POST",
		URL:    "/post",
		Headers: map[string]string{
			"Cookie":       get.Header().Get("Set-Cookie"),
			"X-CSRF-Token": session.Values["csrf_token"].(string),
		},
	})

	getWithToken, getReqWithToken := x.request(g, requestOptions{
		Method: "GET",
		URL:    "/get",
		Headers: map[string]string{
			"Cookie":       get.Header().Get("Set-Cookie"),
			"X-CSRF-Token": session.Values["csrf_token"].(string),
		},
	})

	secondSession, _ := x.csrf.store.Get(getReqWithToken, x.csrf.sessionName)

	assert.Equal(x.T(), session.Values["csrf_token"].(string), secondSession.Values["csrf_token"].(string))
	assert.Equal(x.T(), "OK", get.Body.String())
	assert.Equal(x.T(), http.StatusOK, get.Result().StatusCode)
	assert.Equal(x.T(), "OK", getWithToken.Body.String())
	assert.Equal(x.T(), http.StatusOK, getWithToken.Result().StatusCode)
	assert.Equal(x.T(), "OK", post.Body.String())
	assert.Equal(x.T(), http.StatusOK, post.Result().StatusCode)
}

func (x *CSRFTest) Test_VerifyCSRFMiddleware_NoToken() {
	x.csrf.store = sessions.NewCookieStore([]byte("secret"))
	g := x.NewServer()
	g.GET("/get", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})
	g.POST("/post", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	postWithoutSession, _ := x.request(g, requestOptions{
		Method: "POST",
		URL:    "/post",
	})

	get, getReq := x.request(g, requestOptions{
		Method: "GET",
		URL:    "/get",
	})

	session, _ := x.csrf.store.Get(getReq, x.csrf.sessionName)
	post, _ := x.request(g, requestOptions{
		Method: "POST",
		URL:    "/post",
		Headers: map[string]string{
			"Cookie":       get.Header().Get("Set-Cookie"),
			"X-CSRF-Token": session.Values["csrf_token"].(string),
		},
	})

	postIncorrectToken, _ := x.request(g, requestOptions{
		Method: "POST",
		URL:    "/post",
		Headers: map[string]string{
			"Cookie":       get.Header().Get("Set-Cookie"),
			"X-CSRF-Token": "incorrect token",
		},
	})

	assert.NotEqual(x.T(), "OK", postWithoutSession.Body.String())
	assert.Equal(x.T(), 900, postWithoutSession.Result().StatusCode)
	assert.NotEqual(x.T(), "OK", postIncorrectToken.Body.String())
	assert.Equal(x.T(), 900, postIncorrectToken.Result().StatusCode)
	assert.Equal(x.T(), "OK", get.Body.String())
	assert.Equal(x.T(), http.StatusOK, get.Result().StatusCode)
	assert.Equal(x.T(), "OK", post.Body.String())
	assert.Equal(x.T(), http.StatusOK, post.Result().StatusCode)
}
