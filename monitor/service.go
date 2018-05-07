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
	"github.com/antchfx/htmlquery"
)

type Monitor interface {
	CheckResponse([][]string) (failure bool, failuremsg string, err error)
	GetName() string
}

type Service interface {
}

type service struct {
	callout callout.Service
	alert   alert.Service
	store   Store

	monitors map[string]Monitor

	failing    bool
	cookie     []*http.Cookie
	config     []environment
	currentEnv int
}

func NewService(callout callout.Service, alert alert.Service, store Store, monitors... Monitor) Service {
	s := service{
		alert:   alert,
		callout: callout,
		store:   store,
	}

	s.monitors = map[string]Monitor{}

	for _, m := range monitors {
		s.monitors[m.GetName()] = m
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
			s.alert.SendAlert(context.TODO(), emoji.Sprintf(":warning: %v, count %v", failmsg, count))
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

	guid, err := s.getGuidForMonitor(monitor.Dashboard, monitor.Id)
	if err != nil {
		log.Println(err)
		return
	}
	url := fmt.Sprintf("%v/Prognosis/DashboardView/%v?oTS=#&_=%v", s.getEndpoint(), guid, strconv.FormatInt(time.Now().Unix(), 10))
	req, err := http.NewRequest("GET", url, strings.NewReader(""))
	for _, c := range s.cookie {
		req.AddCookie(c)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var data map[string]interface{}

	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return
	}

	if data == nil {
		err = fmt.Errorf("data nil for dashboard %v, id %v", monitor.Dashboard, monitor.Id)
		return
	}

	//First the root element - thios should include a item called Data
	for _, value := range data {
		v, ok := value.(map[string]interface{})
		if ok {
			for key, t := range v {
				if key == "Data" {
					if len(t.([]interface{})) == 0 {
						err = NoResultsError{Messsage: fmt.Sprintf("Data Length of dashboard %v, graph %v was 0, so no real data", monitor.Dashboard, monitor.Id)}
						return
					}
					d := t.([]interface{})[0].([]interface{})
					if len(d) == 2 {
						err = NoResultsError{Messsage: fmt.Sprintf("Data Length of dashboard %v, graph %v was 2, so no real data", monitor.Dashboard, monitor.Id)}
						return
					}
					var input [][]string
					for _, row := range d[2:] {
						var val []string
						for _, m := range row.([]interface{}) {
							val = append(val, m.(string))
						}
						input = append(input, val)
					}
					monitor := s.monitors[monitor.Type]
					log.Printf(monitor.GetName())
					return monitor.CheckResponse(input)

				}
			}
		}
	}

	err = NoResultsError{fmt.Sprintf("no usable data found for for dashboard %v, id %v", monitor.Dashboard, monitor.Id)}
	return
}

func (s *service) getGuidForMonitor(dashboard, id string) (guid string, err error) {
	url := fmt.Sprintf("%v/Prognosis/Dashboard/Content/%v", s.getEndpoint(), dashboard)
	req, err := http.NewRequest("GET", url, strings.NewReader(""))
	for _, c := range s.cookie {
		req.AddCookie(c)
	}
	resp, err := http.DefaultClient.Do(req)

	doc, err := htmlquery.Parse(resp.Body)

	if err != nil {
		return
	}

	nodes := htmlquery.Find(doc, fmt.Sprintf("//div[@id='%v']", id))
	for _, node := range nodes {
		child := node.FirstChild
		for child.Data != "script" {
			child = child.NextSibling
			if child == nil {
				break
			}
		}
		if child != nil {
			s := child.FirstChild.Data
			lines := strings.Split(s, "\n")
			for _, line := range lines {
				if strings.Index(line, "guid") != -1 {
					guid = line[strings.Index(line, "guid")+5:]
					guid = strings.Replace(guid, "\"", "", -1)
					guid = strings.Replace(guid, ",", "", -1)
					guid = strings.TrimSpace(guid)
					return
				}
			}
		}
	}
	err = fmt.Errorf("no guid found for %v on dashboard %v", id, dashboard)
	return

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

type NoResultsError struct {
	Messsage string
}

func (e NoResultsError) Error() string {
	return "No Results " + e.Messsage
}

func (NoResultsError) RuntimeError() {

}

type config struct {
	Environments []environment
}

type environment struct {
	Address  string
	Monitors []monitors
}

type monitors struct {
	Type, Dashboard, Id, Name string
}

type input struct {
	data map[string]interface{}
}
