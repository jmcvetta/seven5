package seven5

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"seven5/auth"
)

type AuthDispatcher struct {
	provider  []auth.ServiceConnector
	mux       *ServeMux
	prefix    string
	PageMap   auth.PageMapper
	CookieMap CookieMapper
}

//NewAuthDispatcher returns a new auth dispatcher which assumes it is mapped at the prefix provided.
//This should not end with / so mapping at / is passed as "".  The serve mux must be passed because
//because as providers are added to the dispatcher it has to register handlers for them.  Note that
//this dispatcher adds mappings in the mux, based on the prefix provided, so it should not be
//manually added to the ServeMux via the Dispatch() method.
func NewAuthDispatcher(appName string, prefix string, mux *ServeMux) *AuthDispatcher {
	return &AuthDispatcher{
		prefix: prefix,
		mux:    mux,
		PageMap: auth.NewSimplePageMapper("/login.html","/logout.html","/error.html"),
		CookieMap: NewSimpleCookieMapper(appName, NewSimpleSessionManager()),
	}
}

//AddProvider creates the necessary mappings in the AuthDispatcher (and the associated ServeMux)
//handle connectivity with the provider supplied.
func (self *AuthDispatcher) AddProvider(p auth.ServiceConnector) {
	pref := self.prefix + "/" + p.Name() + "/"
	self.mux.Dispatch(pref+"login", self)
	self.mux.Dispatch(pref+"logout", self)
	self.mux.Dispatch(self.callback(p), self)

	self.provider = append(self.provider, p)
}

func (self *AuthDispatcher) Dispatch(mux *ServeMux, w http.ResponseWriter, r *http.Request) *ServeMux {
	split := strings.Split(r.URL.Path, "/")
	if split[0] == "" {
		split = split[1:]
	}
	if len(split) < 3 {
		http.Error(w, fmt.Sprintf("Could not dispatch authentication URL: %s", r.URL), http.StatusNotFound)
		return nil
	}
	if split[0] != self.prefix[1:] {
		http.Error(w, fmt.Sprintf("Could not dispatch authentication URL: %s", r.URL), http.StatusNotFound)
		return nil
	}
	var targ auth.ServiceConnector
	for _, c := range self.provider {
		if c.Name() == split[1] {
			targ = c
			break
		}
	}
	if targ == nil {
		http.Error(w, fmt.Sprintf("Could not dispatch authentication URL: %s", r.URL), http.StatusNotFound)
		return nil
	}
	switch split[2] {
	case auth.LOGIN_URL:
		return self.Login(targ, w, r)
	case auth.LOGOUT_URL:
		return self.Logout(targ, w, r)
	case auth.CALLBACK_URL:
		return self.Callback(targ, w, r)
	}
	
	http.Error(w, fmt.Sprintf("Could not dispatch authentication URL: %s", r.URL), http.StatusNotFound)
	return nil
}

func (self *AuthDispatcher) Login(conn auth.ServiceConnector, w http.ResponseWriter, r *http.Request) *ServeMux {
	state := r.URL.Query().Get(conn.StateValueName())
	http.Redirect(w, r, conn.AuthURL(self.callback(conn), state), http.StatusFound)
	return nil
}

func (self *AuthDispatcher) Logout(conn auth.ServiceConnector, w http.ResponseWriter, r *http.Request) *ServeMux {
	self.CookieMap.RemoveCookie(w)
	self.CookieMap.Destroy(r)
	http.Redirect(w, r, self.PageMap.LogoutLandingPage(conn), http.StatusTemporaryRedirect)
	return nil
}

func (self *AuthDispatcher) Callback(conn auth.ServiceConnector, w http.ResponseWriter, r *http.Request) *ServeMux {
	code := r.URL.Query().Get(conn.CodeValueName())
	e := r.URL.Query().Get(conn.ErrorValueName())
	
	if e != "" {
		http.Redirect(w, r, self.PageMap.ErrorPage(conn, e), http.StatusTemporaryRedirect)
		return nil
	}
	return self.Connect(conn, code, w, r)
}

func (self *AuthDispatcher) callback(conn auth.ServiceConnector) string {
	return self.prefix + "/" + conn.Name() + "/" + auth.CALLBACK_URL
}

func (self *AuthDispatcher) Connect(conn auth.ServiceConnector, code string, w http.ResponseWriter, r *http.Request) *ServeMux {
	trans, err := conn.ExchangeForToken(self.callback(conn), code)
	if err != nil {
		error_msg := fmt.Sprintf("unable to finish the token exchange with %s: %s", conn.Name(), err)
		http.Redirect(w, r, self.PageMap.ErrorPage(conn, error_msg), http.StatusTemporaryRedirect)
		return nil
	}
	state := r.URL.Query().Get(conn.StateValueName())
	session, err := self.CookieMap.Generate(conn.Name(), trans, r, code)
	if err != nil {
		error_msg := fmt.Sprintf("failed to create session")
		http.Redirect(w, r, self.PageMap.ErrorPage(conn, error_msg), http.StatusTemporaryRedirect)
		return nil
	}
	self.CookieMap.AssociateCookie(w, session)
	http.Redirect(w, r, self.PageMap.LoginLandingPage(conn, state, code), http.StatusTemporaryRedirect)
	return nil
}

func toWebUIPath(s string) string {
	return fmt.Sprintf("/out%s", s)
}

func UDID() string {
	f, err := os.Open("/dev/urandom")
	if err != nil {
		panic(fmt.Sprintf("failed to get /dev/urandom! %s", err))
	}
	b := make([]byte, 16)
	_, err = f.Read(b)
	if err != nil {
		panic(fmt.Sprintf("failed to read  16 bytes from /dev/urandom! %s", err))
	}
	f.Close()
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}