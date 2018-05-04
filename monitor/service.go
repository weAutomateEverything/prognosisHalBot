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
	"encoding/json"
	"github.com/pkg/errors"
	"github.com/kyokomi/emoji"
)

type Monitor interface {
	checkResponse(r *http.Response) (failure bool, failuremsg string, err error)
}

type Service interface {
}

type service struct {
	callout callout.Service
	alert   alert.Service
	store   Store

	failing    bool
	cookie     []*http.Cookie
	config     []environment
	currentEnv int
}

func NewService(callout callout.Service, alert alert.Service, store Store) Service {
	s := service{
		alert:   alert,
		callout: callout,
		store:   store,
	}

	cfg := os.Getenv("CONFIG_URL")
	if cfg == "" {
		panic("CONFIG_URL environment variable is not set.")
	}
	var configs []environment

	resp, err := http.Get(cfg)
	if err != nil {
		panic(err)
	}

	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&configs)
	if err != nil {
		panic(err)
	}

	s.config = configs

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

	for _, monitor := range s.config[s.currentEnv].Monitors {

		failed, failmsg, err := s.checkMonitor(monitor)

		//If there is an error fetching data, lets handle it, but not use the results to determine the system health
		if err != nil {
			//If the error is not a NoResultsError, it means we have another technical error
			s.alert.SendError(context.TODO(), err)
			continue
		}

		//If the current run has failed, but we are not already in a failed state, invoke callout. This is to prevent callout from being invoked for every error.
		if failed {
			err := s.store.increaseCount(monitor.Id)
			if err != nil {
				log.Println(err)
				s.alert.SendError(context.TODO(), err)
			}
			count, err := s.store.getCount(monitor.Id)

			if err != nil {
				log.Println(err)
				s.alert.SendError(context.TODO(), err)
			}
			s.alert.SendAlert(context.TODO(),emoji.Sprintf(":warning: %v, count %v",failmsg,count))
			if count == 10 {
				s.callout.InvokeCallout(context.TODO(), "Prognosis Issue Detected", failmsg)
			}
			return
		} else {
			s.store.zeroCount(monitor.Id)
		}
	}

	req, _ := http.NewRequest("GET", fmt.Sprintf("%v/Prognosis/Logout", s.getEndpoint()), strings.NewReader(""))
	for _, c := range s.cookie {
		req.AddCookie(c)
	}
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()
}

func (s *service) getLoginCookie() error {
	for i, x := range s.config {
		log.Println("Logging in")
		v := url.Values{}
		v.Add("UserName", getUsername())
		v.Add("Password", getPassword())
		v.Add("Destination", "View Systems")
		req, err := http.NewRequest("POST", fmt.Sprintf("%v/Prognosis/Login?returnUrl=/Prognosis/", x.Address), strings.NewReader(v.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := http.DefaultClient.Do(req)

		if err != nil {
			s.alert.SendError(context.TODO(), err)
			continue
		}
		s.currentEnv = i
		defer resp.Body.Close()
		s.cookie = resp.Cookies()
		return nil
	}
	return errors.New("Unable to find an environment to successfully log into")

}

func (s *service) checkMonitor(monitor monitors) (failing bool, message string, err error) {
	url := fmt.Sprintf("%v/Prognosis/DashboardView/%v?oTS=#&_=%v", s.getEndpoint(), monitor.Id, strconv.FormatInt(time.Now().Unix(), 10))
	req, err := http.NewRequest("GET", url, strings.NewReader(""))
	for _, c := range s.cookie {
		req.AddCookie(c)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	return s.findMonitor(monitor).checkResponse(resp)
}

func (s *service) findMonitor(m monitors) Monitor {

	switch m.Type {
	case "FailureRate":
		{
			return NewFailureRateMonitor(s.store)
		}
	case "Code91":
		{
			return NewResponseCode91Monitor(s.store)
		}
	}
	panic(fmt.Sprintf("Unable to find monitor type %v", m.Type))
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

func (s service) getEndpoint() string {
	return s.config[s.currentEnv].Address
}

func getUsername() string {
	return os.Getenv("PROGNOSIS_USERNAME")

}

func getPassword() string {
	return os.Getenv("PROGNOSIS_PASSWORD")

}

type noResultsError struct {
	messsage string
}

func (e noResultsError) Error() string {
	return "No Results "+e.messsage
}

func (noResultsError) RuntimeError() {

}

type config struct {
	Environments []environment
}

type environment struct {
	Address  string
	Monitors []monitors
}

type monitors struct {
	Type, Id, Name string
}
