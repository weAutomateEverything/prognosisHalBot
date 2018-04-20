package monitor

import (
	"net/http"
	"os"
	"time"
	"crypto/tls"
	"github.com/ernesto-jimenez/httplogger"
	"net/url"
	"strings"
	"log"
	"github.com/weAutomateEverything/go2hal/callout"
	"golang.org/x/net/context"
	"strconv"
	"github.com/weAutomateEverything/go2hal/alert"
	"fmt"
)

type Monitor interface {
	getEndpoint() string
	checkResponse(r *http.Response) (failure bool, failuremsg string, err error)
}

type Service interface {
}

type service struct {
	monitors []Monitor
	callout  callout.Service
	alert    alert.Service

	failing bool
	cookie  []*http.Cookie
}

func NewService(callout callout.Service, alert alert.Service, monitors ...Monitor) Service {
	s := service{
		monitors: monitors,
		alert:    alert,
	}
	go func() { s.runChecks() }()

	return s
}

func (s *service) runChecks() {
	for true {
		s.checkPrognosis()
		time.Sleep(10 * time.Second)
	}
}

func (s *service) checkPrognosis() {
	log.Println("Starting Prognosis")
	logger := newLogger()
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	http.DefaultClient.Transport = httplogger.NewLoggedTransport(http.DefaultTransport, logger)
	http.DefaultClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	//Login - get the cookie for auth
	s.getLoginCookie()

	allOk := true

	for _, monitor := range s.monitors {

		failed, failmsg, err := s.checkMonitor(monitor)

		//If there is an error fetching data, lets handle it, but not use the results to determine the system health
		if err != nil {
			allOk = false
			//If the error is not a NoResultsError, it means we have another technical error
			if _, ok := err.(noResultsError); !ok {
				s.alert.SendError(context.TODO(), err)
			}
			continue
		}

		//If the current run has failed, but we are not already in a failed state, invoke callout. This is to prevent callout from being invoked for every error.
		if failed && !s.failing {
			s.failing = true
			s.callout.InvokeCallout(context.TODO(), "Prognosis Issue Detected", failmsg)
			return
		}
	}

	//We have looked through all the tests - everythign appears fine. So lets set the system into a passing state
	if allOk {
		s.failing = false
	}

	req, _ := http.NewRequest("GET", fmt.Sprintf("%v/Prognosis/Logout",getEndpoint()), strings.NewReader(""))
	for _, c := range s.cookie {
		req.AddCookie(c)
	}
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()
}

func (s *service) getLoginCookie() error {
	log.Println("Logging in")
	v := url.Values{}
	v.Add("UserName", getUsername())
	v.Add("Password", getPassword())
	v.Add("Destination", "View Systems")
	req, err := http.NewRequest("POST", fmt.Sprintf("%v/Prognosis/Login?returnUrl=/Prognosis/", getEndpoint()), strings.NewReader(v.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		return err
	}
	defer resp.Body.Close()
	s.cookie = resp.Cookies()
	return nil
}

func (s *service) checkMonitor(monitor Monitor) (failing bool, message string, err error) {
	req, err := http.NewRequest("GET", getEndpoint()+monitor.getEndpoint()+"?oTS=%23&_="+strconv.FormatInt(time.Now().Unix(), 10), strings.NewReader(""))
	for _, c := range s.cookie {
		req.AddCookie(c)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	return monitor.checkResponse(resp)
}

type httpLogger struct {
	log *log.Logger
}

func newLogger() *httpLogger {
	return &httpLogger{
		log: log.New(os.Stderr, "log - ", log.LstdFlags),
	}
}

func (l *httpLogger) LogRequest(req *http.Request) {
	l.log.Printf(
		"Request %s %s",
		req.Method,
		req.URL.String(),
	)
}

func (l *httpLogger) LogResponse(req *http.Request, res *http.Response, err error, duration time.Duration) {
	duration /= time.Millisecond
	if err != nil {
		l.log.Println(err)
	} else {
		l.log.Printf(
			"Response method=%s status=%d durationMs=%d %s",
			req.Method,
			res.StatusCode,
			duration,
			req.URL.String(),
		)
	}
}

func getEndpoint() string {
	return os.Getenv("PROGNOSIS_ENDPOINT")
}

func getUsername() string {
	return os.Getenv("PROGNOSIS_USERNAME")

}

func getPassword() string {
	return os.Getenv("PROGNOSIS_PASSWORD")

}

type noResultsError struct {
}

func (noResultsError) Error() string {
	return "No Results"
}

func (noResultsError) RuntimeError() {

}
