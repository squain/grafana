package middleware

import (
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/go-macaron/session"
	_ "github.com/go-macaron/session/memcache"
	_ "github.com/go-macaron/session/mysql"
	_ "github.com/go-macaron/session/postgres"
	_ "github.com/go-macaron/session/redis"
	"github.com/grafana/grafana/pkg/log"
	"github.com/grafana/grafana/pkg/setting"
	"gopkg.in/macaron.v1"
)

const (
	SESS_KEY_USERID       = "uid"
	SESS_KEY_OAUTH_STATE  = "state"
	SESS_KEY_APIKEY       = "apikey_id" // used for render requests with api keys
	SESS_KEY_LASTLDAPSYNC = "last_ldap_sync"
)

var sessionManager *session.Manager
var sessionOptions *session.Options
var startSessionGC func()
var getSessionCount func() int
var sessionLogger = log.New("session")

func Sessioner(options *session.Options) macaron.Handler {
	return func(ctx *Context) {
		ctx.Resp.Before(func(macaron.ResponseWriter) {
			// Need to write session to cookie before anything else
			ctx.Session.Write(ctx)
		})
		ctx.Next()
	}
}

func GetSession() SessionStore {
	return &CookieSessionStore{
		cookieName:   setting.SessionOptions.CookieName,
		cookieSecure: setting.SessionOptions.Secure,
		cookieMaxAge: setting.SessionOptions.CookieLifeTime,
		cookieDomain: setting.SessionOptions.Domain,
		cookiePath:   setting.SessionOptions.CookiePath,
		data:         make(map[string]string),
	}
}

type SessionStore interface {
	SetString(string, string)
	GetString(string) (string, bool)

	SetInt64(string, int64)
	GetInt64(string) (int64, bool)

	Start(*Context) error
	Destory(*Context) error
	Write(*Context) error
}

type CookieSessionStore struct {
	cookieName   string
	cookieSecure bool
	cookiePath   string
	cookieDomain string
	cookieMaxAge int
	data         map[string]string
	value        string
}

func (s *CookieSessionStore) Start(ctx *Context) error {
	cookieString := ctx.GetCookie(s.cookieName)
	if len(cookieString) > 0 {
		s.value = cookieString
		sessionLogger.Debug("Start()", "cookie", cookieString)
	}
	return nil
}

func (s *CookieSessionStore) Write(ctx *Context) error {
	// if no data clear cookie
	if len(s.data) == 0 {
		cookieString := ctx.GetCookie(s.cookieName)
		if len(cookieString) == 0 {
			return nil
		}

		cookie := &http.Cookie{
			Name:     s.cookieName,
			Path:     s.cookiePath,
			HttpOnly: true,
			Expires:  time.Now(),
			MaxAge:   -1,
		}

		http.SetCookie(ctx.Resp, cookie)
		sessionLogger.Debug("Write() Clearing Empty Session")
		return nil
	}

	tmpUrl := &url.URL{}
	queryVars := tmpUrl.Query()
	for key, value := range s.data {
		queryVars.Add(key, value)
	}

	// update cookie
	cookie := &http.Cookie{
		Name:     s.cookieName,
		Value:    queryVars.Encode(),
		Path:     s.cookiePath,
		HttpOnly: true,
		Secure:   s.cookieSecure,
		Domain:   s.cookieDomain,
	}

	if s.cookieMaxAge >= 0 {
		cookie.MaxAge = s.cookieMaxAge
	}

	sessionLogger.Debug("Session.Write", "cookie", cookie.Value)
	http.SetCookie(ctx.Resp, cookie)
	ctx.Req.AddCookie(cookie)
	return nil
}

func (s *CookieSessionStore) SetString(k string, v string) {
	sessionLogger.Info("Set", "key", k, "value", v)
	s.data[k] = v
}

func (s *CookieSessionStore) GetString(k string) (string, bool) {
	sessionLogger.Info("Get", "key", k)
	value, exist := s.data[k]
	return value, exist
}

func (s *CookieSessionStore) SetInt64(k string, v int64) {
	sessionLogger.Info("Set", "key", k, "value", v)
	s.data[k] = strconv.FormatInt(v, 10)
}

func (s *CookieSessionStore) GetInt64(k string) (int64, bool) {
	sessionLogger.Info("Get", "key", k)
	if value, exist := s.data[k]; exist {
		if intValue, err := strconv.ParseInt(value, 10, 64); err != nil {
			return intValue, true
		}
	}
	return 0, false
}

func (s *CookieSessionStore) Destory(c *Context) error {
	s.data = make(map[string]string)
	return nil
}
