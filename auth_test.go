package seven5

import (
	"code.google.com/p/gomock/gomock"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"seven5/auth"
	"strings"
	"testing"
)

const (
	SCOPE  = "https://www.googleapis.com/auth/userinfo.profile https://www.googleapis.com/auth/userinfo.email "
	PROMPT = "auto" //can be "force" or "auto"
)

/*-------------------------------------------------------------------------------*/

func TestHerokuName(T *testing.T) {
	n := auth.HerokuName("fleazil")
	if n != "https://fleazil.herokuapp.com" {
		fmt.Printf("Unexpected name for heroku app fleazil: %s", n)
	}
}

/*-------------------------------------------------------------------------------*/
var (
	//signal value for stopping redirect processing
	stopProcessing = errors.New("STOP OR I'LL TASE YA")
	returl         = "/fart/google/oauth2callback"
	id             = "painful"
	seekret        = "pustules"
	errorText      = "no such luck with google on transport to get token"
	badTransport   = errors.New(errorText)
	three          = "/3.html"
	two            = "/2.html"
	appName        = "possesonbroadway"
)

/*-------------------------------------------------------------------------------*/
func TestGoogleLogin(t *testing.T) {
	port := 8201

	//create controller for all mocks
	ctrl := gomock.NewController(t)
	//check mocks at end
	defer ctrl.Finish()

	//the key values
	st := "/frob bob"
	loginurl := "/fart/google/login"
	code := "barfly"
	one := "/1.html"
	sid := "id of session, sid vicious?"

	//authconn is a wrapper around the google auth connector with all mock methods, except AuthURL
	//pm is a mock for testing that we get a call to LoginLandingPage
	pm := auth.NewMockPageMapper(ctrl)
	sm := NewMockSessionManager(ctrl)
	cm := NewSimpleCookieMapper(appName, sm)
	serveMux, authconn := createDispatcherWithMocks(ctrl, pm, cm)

	session := NewMockSession(ctrl)
	session.EXPECT().SessionId().Return(sid).AnyTimes()
	//when we succeed at logging in, it filters down to the session
	sm.EXPECT().Generate().Return(session, nil)

	//consumed by the google object under test
	deploy := auth.NewMockDeploymentEnvironment(ctrl)
	deploy.EXPECT().RedirectHost(gomock.Any()).Return(fmt.Sprintf("http://localhost:%d", port))

	detail := auth.NewMockOauthClientDetail(ctrl)
	detail.EXPECT().ClientId(gomock.Any()).Return(id)
	detail.EXPECT().ClientSecret(gomock.Any()).Return(seekret)

	//we are testing the AuthURL method, and NOT testing ExchangeForToken() as it requires a
	//real network and a real client id and seekret
	google := auth.NewGoogleAuthConnector(SCOPE, PROMPT, detail, deploy)

	//these are just accessing the constants, so don't care how many times
	//authconn.EXPECT().Name().Return("google").AnyTimes()
	authconn.EXPECT().StateValueName().Return("state").AnyTimes()
	authconn.EXPECT().ErrorValueName().Return("error").AnyTimes()
	authconn.EXPECT().CodeValueName().Return("code").AnyTimes()

	//this is actually under test, the google.AuthURL method
	authconn.EXPECT().AuthURL(returl, st).Return(google.AuthURL(returl, st))

	//this is mocked out because it has the side effect of a network call... we can use the mocks
	//to return an error which we do in the second case
	gomock.InOrder(
		authconn.EXPECT().ExchangeForToken(returl, code).Return(nil, nil),
		authconn.EXPECT().ExchangeForToken(returl, code).Return(nil, badTransport),
	)

	//testing that page mapper's login method gets called during the login process to generate the
	//final web page to land on
	pm.EXPECT().LoginLandingPage(authconn, st, code).Return(one)
	pm.EXPECT().ErrorPage(authconn, gomock.Any()).Return(three)

	go func() {
		http.ListenAndServe(fmt.Sprintf(":%d", port), serveMux)
	}()

	//we need to compute a return url (stage 2) because we expect to see at redir of stage 1
	returnURLBase := fmt.Sprintf("http://localhost:%d%s", port, returl)

	//we need to compute a login url, with a state value to make sure it is propagated all the
	//way through to LoginLandingPage()
	v := url.Values{
		"state": []string{st},
	}
	loginURL, err := url.Parse(fmt.Sprintf("http://localhost:%d%s?%s", port, loginurl, v.Encode()))
	if err != nil {
		t.Fatalf("Can't understand url: %s", err)
	}

	//setup client to not really do redirects so we can look at what's going on
	client := new(http.Client)
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		checkRedirValues(t, "phase 1 of login", via, map[string][]string{
			"path":       []string{req.URL.Path, auth.GOOGLE_AUTH_URL_PATH},
			"host":       []string{req.URL.Host, auth.GOOGLE_AUTH_URL_HOST[len("https://"):]},
			"scheme":     []string{req.URL.Scheme, "https"},
			"state":      []string{req.URL.Query().Get("state"), st},
			"client_id":  []string{req.URL.Query().Get("client_id"), id},
			"via url[0]": []string{via[0].URL.String(), loginURL.String()},
		})
		if !strings.HasPrefix(req.URL.Query().Get("redirect_uri"), returnURLBase) {
			t.Errorf("Serious problems understanding the callback uri: %s", req.URL.Query().Get("redirect_uri"))
		}
		return stopProcessing
	}

	createReqAndDo(t, client, loginURL.String(),nil)

	// next stage is to test that if we get the callabck we land on the right page
	// in the right state... compute a URL like google would send us
	v = url.Values{
		"code":  []string{code},
		"state": []string{st},
		"error": []string{},
	}

	returnURL, err := url.Parse(fmt.Sprintf("%s?%s", returnURLBase, v.Encode()))
	if err != nil {
		t.Fatalf("Can't understand url: %s", err)
	}

	//now check the value we redirect back to on successful login... this simulates what google
	//would send back to us after successful handshake... again, we don't allow the redir
	//to be processed
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		checkRedirValues(t, "phase 2 of login", via, map[string][]string{
			"path":       []string{req.URL.Path, one},
			"host":       []string{req.URL.Host, fmt.Sprintf("localhost:%d", port)},
			"scheme":     []string{req.URL.Scheme, "http"},
			"state":      []string{req.URL.Query().Get("state"), ""}, //sanity
			"via url[0]": []string{via[0].URL.String(), returnURL.String()},
		})
		return stopProcessing
	}

	//make sure cookie manager sent us something
	resp := createReqAndDo(t, client, returnURL.String(),nil)
	found := false
	for k, v := range resp.Header {
		if k == "Set-Cookie" {
			found = true
			p := strings.Split(v[0], ";")
			if strings.Index(p[0], cm.CookieName()) == -1 {
				t.Errorf("Found a cookie but expected name '%s' but couldn't find it in header: %s",
					cm.CookieName(), p[0])
			}
			if strings.Index(p[0], sid) == -1 {
				t.Errorf("Found a cookie but expected value '%s' but couldn't find it in header: %s",
					sid, p[0])
			}
		}
	}
	if !found {
		t.Errorf("Didn't find cookie '%s'", cm.CookieName())
	}
	//this tests that if the transport connection to the provider fails, we get the error page
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		checkRedirValues(t, "phase 2 (bad network)", via, map[string][]string{
			"path":       []string{req.URL.Path, three},
			"host":       []string{req.URL.Host, fmt.Sprintf("localhost:%d", port)},
			"via url[0]": []string{via[0].URL.String(), returnURL.String()},
			//error parameter is checked in a diff test
		})
		return stopProcessing
	}

	createReqAndDo(t, client, returnURL.String(), nil)

}

/*-------------------------------------------------------------------------------*/
func checkRedirValues(t *testing.T, name string, via []*http.Request, tbl map[string][]string) {
	for k, v := range tbl {
		if v[0] != v[1] {
			t.Errorf("mismatch in redirect at %s, field named '%s': got '%s' but expected '%s'",
				name, k, v[0], v[1])
		}
	}
	if len(via) != 1 {
		t.Fatalf("Unexpected number of previous requests, expected 1 but got %d", len(via))
	}

}

/*-------------------------------------------------------------------------------*/
func createReqAndDo(t *testing.T, client *http.Client, targ string, c *http.Cookie) *http.Response {
	req, err := http.NewRequest("GET", targ, nil)
	if err != nil {
		t.Fatalf("failed to create GET request (target was %s): %s", targ, err)
	}
	if c!=nil {
		req.AddCookie(c)
	}
	result, err := client.Do(req)
	uerr := err.(*url.Error)
	if uerr.Err != stopProcessing {
		t.Fatalf("failed to stop http request from redirection (target was %s): %s", targ, err)
	}
	return result
}

/*-------------------------------------------------------------------------------*/
func createDispatcherWithMocks(ctrl *gomock.Controller, pm auth.PageMapper, cm CookieMapper) (*ServeMux, *auth.MockServiceConnector) {
	authconn := auth.NewMockServiceConnector(ctrl)

	//real serve mux so the dispatching really works with an HTTP conn
	serveMux := NewServeMux(nil)

	//we use /fart because we don't want to end up with dependencies on /rest, the standard
	disp := NewAuthDispatcher(appName, "/fart", serveMux)
	//finesse page mapper and cookie into the struct
	disp.PageMap = pm
	disp.CookieMap = cm

	//put our mostly stub auth connector into the URL space and we don't care how many times
	//it gets asked its name
	authconn.EXPECT().Name().Return("google").AnyTimes()
	disp.AddProvider(authconn)

	return serveMux, authconn
}

/*-------------------------------------------------------------------------------*/
func TestCallbackError(t *testing.T) {
	port := 8202

	//create controller for all mocks
	ctrl := gomock.NewController(t)
	//check mocks at end
	defer ctrl.Finish()

	//key values
	loser := "you are a loser"
	state := "jabba da hut/:)" //make sure we are decoding correctly by adding strange chars

	pageMapper := auth.NewSimplePageMapper(three, "notused", "notused")
	cookieMapper := NewMockCookieMapper(ctrl)
	serveMux, authConn := createDispatcherWithMocks(ctrl, pageMapper, cookieMapper)
	go func() {
		http.ListenAndServe(fmt.Sprintf(":%d", port), serveMux)
	}()

	//don't care about the cookie name
	cookieMapper.EXPECT().CookieName().Return("my_chef").AnyTimes()

	//just to get the constants
	authConn.EXPECT().ErrorValueName().Return("error")
	authConn.EXPECT().CodeValueName().Return("code")

	// this is what happens when google refuses
	v := url.Values{
		//no code!
		"state": []string{state},
		"error": []string{loser},
	}

	returnURLHost := fmt.Sprintf("localhost:%d", port)
	returnURL, err := url.Parse(fmt.Sprintf("http://%s%s?%s", returnURLHost, returl, v.Encode()))
	if err != nil {
		t.Fatalf("Can't understand url: %s", err)
	}

	client := new(http.Client)
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		checkRedirValues(t, "error from goog", via, map[string][]string{
			"path":       []string{req.URL.Path, three},
			"host":       []string{req.URL.Host, returnURLHost},
			"state":      []string{req.URL.Query().Get("state"), ""},
			"error":      []string{req.URL.Query().Get("error"), loser},
			"service":    []string{req.URL.Query().Get("service"), "google"},
			"via url[0]": []string{via[0].URL.String(), returnURL.String()},
		})
		return stopProcessing
	}
	resp := createReqAndDo(t, client, returnURL.String(),nil)
	for k, v := range resp.Header {
		if k == "Set-Cookie" {
			t.Errorf("Should not have set cookie on error: %s\n", v[0])
		}
	}

}

/*-------------------------------------------------------------------------------*/
func TestLogout(t *testing.T) {
	port := 8203

	//create controller for all mocks
	ctrl := gomock.NewController(t)
	//check mocks at end
	defer ctrl.Finish()

	pageMapper := auth.NewSimplePageMapper("notused", "notused", two)
	sm := NewMockSessionManager(ctrl)
	cookieMapper := NewSimpleCookieMapper(appName, sm)
	serveMux, _ := createDispatcherWithMocks(ctrl, pageMapper, cookieMapper)

	sm.EXPECT().Destroy(gomock.Any()).Return(nil)

	go func() {
		http.ListenAndServe(fmt.Sprintf(":%d", port), serveMux)
	}()

	logoutURLHost := fmt.Sprintf("localhost:%d", port)
	logoutURL, err := url.Parse(fmt.Sprintf("http://%s%s", logoutURLHost, "/fart/google/logout"))
	if err != nil {
		t.Fatalf("Can't understand url: %s", err)
	}

	client := new(http.Client)
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		checkRedirValues(t, "error from goog", via, map[string][]string{
			"path":       []string{req.URL.Path, two},
			"host":       []string{req.URL.Host, logoutURLHost},
			"via url[0]": []string{via[0].URL.String(), logoutURL.String()},
		})
		return stopProcessing
	}
	resp := createReqAndDo(t, client, logoutURL.String(),
		&http.Cookie{
			Name:      cookieMapper.CookieName(),
			Value:     "forty-series-tires",
		})
	for k, v := range resp.Header {
		if k == "Set-Cookie" {
			p := strings.Split(v[0], ";")
			for _, piece := range p {
				if strings.Index(piece, cookieMapper.CookieName()) != -1 {
					if strings.TrimSpace(piece) != cookieMapper.CookieName()+"=" {
						t.Errorf("Cookie not destroyed properly! '%s'", piece)
					}
				}
				if strings.Index(piece, "Max-Age") != -1 {
					if strings.TrimSpace(piece) != "Max-Age=0" {
						t.Errorf("Cookie not destroyed properly! '%s'", piece)
					}
				}
			}
		}
	}

}
