package monitor

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/antchfx/htmlquery"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/ernesto-jimenez/httplogger"
	"github.com/kyokomi/emoji"
	"github.com/weAutomateEverything/prognosisHalBot/client"
	"github.com/weAutomateEverything/prognosisHalBot/client/alert"
	"github.com/weAutomateEverything/prognosisHalBot/client/operations"
	"github.com/weAutomateEverything/prognosisHalBot/models"
	"golang.org/x/net/context"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Monitor interface {
	CheckResponse([][]string) (failure bool, failuremsg string, err error)
	GetName() string
}

type Service interface {
}

type service struct {
	store Store
	hal   *client.GO2HAL

	monitors map[string]Monitor

	failing    bool
	cookie     []*http.Cookie
	config     environment
	currentEnv int

	techErrCount int
}

func NewService(hal *client.GO2HAL, store Store, monitors ...Monitor) Service {
	s := service{
		store: store,
		hal:   hal,
	}

	s.monitors = map[string]Monitor{}

	for _, m := range monitors {
		s.monitors[m.GetName()] = m
	}

	cfg := os.Getenv("CONFIG_URL")
	if cfg == "" {
		panic("CONFIG_URL environment variable is not set.")
	}
	var configs environment
	logger := NewLogger()

	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	http.DefaultClient.Transport = httplogger.NewLoggedTransport(http.DefaultTransport, logger)
	http.DefaultClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

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
	logger := NewLogger()
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	http.DefaultClient.Transport = httplogger.NewLoggedTransport(http.DefaultTransport, logger)
	http.DefaultClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	//Login - get the cookie for auth
	s.getLoginCookie()

	for true {
		s.checkPrognosis()
		time.Sleep(10 * time.Second)
	}
}

func (s *service) checkPrognosis() {
	log.Println("Starting Prognosis")

	for _, monitor := range s.config.Monitors {

		failed, failmsg, err := s.checkMonitor(monitor)

		//If there is an error fetching data, lets handle it, but not use the results to determine the system health
		if err != nil {
			_, ok := err.(NoResultsError)
			if !ok {
				s.techErrCount++
				if s.techErrCount == 10 {
					s.sendMessage("10 failures detected. Attempting login to find a new host", monitor.Group)
					continue
				}
			}
			s.techErrCount = 0
			//If the error is not a NoResultsError, it means we have another technical error
			s.sendMessage(err.Error(), monitor.Group)
			continue
		}
		s.techErrCount = 0

		if failed {
			err := s.store.IncreaseCount(monitor.Id)
			if err != nil {
				log.Println(err)
				s.sendMessage(err.Error(), monitor.Group)
			}
			count, t, err := s.store.GetCount(monitor.Id)
			d := time.Since(t)

			if err != nil {
				log.Println(err)
				s.sendMessage(err.Error(), monitor.Group)
			}
			//Ignore the first 2 errors - this should make the alerts less noisy
			if count > 3 {
				s.sendMessage(emoji.Sprintf(":x: %v. Error has been occurring for %v.", failmsg, d.String()), monitor.Group)
			}

			//After 15 alerts, lets invoke callout
			if count == 15 {
				s.hal.Operations.InvokeCallout(&operations.InvokeCalloutParams{
					Chatid:  monitor.Group,
					Context: getTimeout(),
					Body: &models.SendCalloutRequest{
						Message: aws.String(fmt.Sprintf("Prognosis Issue Detected. %v", failmsg)),
						Title:   aws.String(failmsg),
					},
				})
			}
			return
		} else {
			count, t, _ := s.store.GetCount(monitor.Id)
			d := time.Since(t)
			if count > 3 {
				s.sendMessage(emoji.Sprintf(":white_check_mark: No issues detected. Errors occurred for %v", d.String()), monitor.Group)
			}
			s.store.ZeroCount(monitor.Id)
		}
	}
}

func (s *service) getLoginCookie() error {
	for true {
		for i, x := range s.config.Address {
			log.Println("Logging in")
			v := url.Values{}
			v.Add("UserName", getUsername())
			v.Add("Password", getPassword())
			v.Add("Destination", "View Systems")
			req, err := http.NewRequest("POST", fmt.Sprintf("%v/Prognosis/Login?returnUrl=/Prognosis/", x), strings.NewReader(v.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			resp, err := http.DefaultClient.Do(req)

			if err != nil {
				s.sendMessage("prognosis error - "+err.Error(), getErrorGroup())
				continue
			}
			if len(resp.Cookies()) == 0 {
				s.sendMessage("Prognosis error - No cookie found on response", getErrorGroup())
				continue
			}
			s.currentEnv = i
			defer resp.Body.Close()
			s.cookie = resp.Cookies()
			return nil
		}
		s.sendMessage("Unable to successfully log into prognosis... will try again in 60 seconds", getErrorGroup())
		time.Sleep(60 * time.Second)
	}
	return nil

}

func (s *service) checkMonitor(monitor monitors) (failing bool, message string, err error) {

	guid, err := s.getGuidForMonitor(monitor)
	if err != nil {
		log.Println(err)
		return
	}
	if monitor.ObjectType == "" {
		monitor.ObjectType = "#"
	}
	url := fmt.Sprintf("%v/Prognosis/DashboardView/%v",
		s.getEndpoint(),
		guid,
	)
	req, err := http.NewRequest("GET", url, strings.NewReader(""))
	for _, c := range s.cookie {
		req.AddCookie(c)
	}

	count := 0
httpDO:
	for true {
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return false, "", err
		}
		var data map[string]interface{}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			err = fmt.Errorf("error reading body for dashboard %v, id %v, error %v", monitor.Dashboard, monitor.Id, err.Error())
			return false, "", err
		}

		log.Println(string(body))

		err = json.Unmarshal(body, &data)

		resp.Body.Close()
		if err != nil {
			return false, "", err
		}

		if data == nil {
			err = fmt.Errorf("data nil for dashboard %v, id %v", monitor.Dashboard, monitor.Id)
			return false, "", err
		}

		//First the root element - thios should include a item called Data
		for _, value := range data {
			v, ok := value.(map[string]interface{})
			if ok {
				for key, t := range v {
					if key == "Data" {
						if len(t.([]interface{})) == 0 {
							count++
							//Sometimes, it takes prognosis a while to wake up... so the first 10 no data we can ignore
							if count == 10 {
								err = NoResultsError{Messsage: fmt.Sprintf("Data Length of dashboard %v, graph %v was 0, so no real data", monitor.Dashboard, monitor.Id)}
								return false, "", err
							}
							time.Sleep(2 * time.Second)
							continue httpDO
						}
						d := t.([]interface{})[0].([]interface{})
						if len(d) == 2 {
							err = NoResultsError{Messsage: fmt.Sprintf("Data Length of dashboard %v, graph %v was 2, so no real data", monitor.Dashboard, monitor.Id)}
							return false, "", err
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
	}

	err = NoResultsError{fmt.Sprintf("no usable data found for for dashboard %v, id %v", monitor.Dashboard, monitor.Id)}
	return
}

func (s *service) getGuidForMonitor(monitor monitors) (guid string, err error) {
	url := fmt.Sprintf("%v/Prognosis/Dashboard/Content/%v", s.getEndpoint(), monitor.Dashboard)
	req, err := http.NewRequest("GET", url, strings.NewReader(""))
	for _, c := range s.cookie {
		req.AddCookie(c)
	}
	resp, err := http.DefaultClient.Do(req)

	doc, err := htmlquery.Parse(resp.Body)

	if err != nil {
		return
	}

	nodes := htmlquery.Find(doc, fmt.Sprintf("//div[@id='%v']", monitor.Id))
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
	msg := fmt.Sprintf("no guid found for %v on dashboard %v. Restarting Bot", monitor.Id, monitor.Dashboard)
	s.sendMessage(msg, monitor.Group)
	panic(msg)
	return

}

func (s *service) sendMessage(message string, group int64) {
	message = strings.Replace(message, "_", " ", -1)
	resp, err := s.hal.Alert.SendTextAlert(&alert.SendTextAlertParams{
		Context: getTimeout(),
		Chatid:  group,
		Message: aws.String(message),
	})
	if err != nil {
		log.Println(err.Error())
	}
	if resp != nil {
		log.Println(resp.Error())
	}
}

type httpLogger struct {
	log *log.Logger
}

func NewLogger() *httpLogger {
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
	return s.config.Address[s.currentEnv]
}

func getUsername() string {
	return os.Getenv("PROGNOSIS_USERNAME")

}

func getPassword() string {
	return os.Getenv("PROGNOSIS_PASSWORD")
}

func getErrorGroup() int64 {
	int, err := strconv.ParseInt(os.Getenv("ERROR_GROUP"), 10, 64)
	if err != nil {
		log.Println(err)
		return 0
	}
	return int
}

type NoResultsError struct {
	Messsage string
}

func (e NoResultsError) Error() string {
	return "No Results " + e.Messsage
}

func (NoResultsError) RuntimeError() {

}

type environment struct {
	Address  []string
	Monitors []monitors
}

type monitors struct {
	Type, Dashboard, Id, Name, ObjectType string
	Group                                 int64
}

func getTimeout() context.Context {
	c, _ := context.WithTimeout(context.TODO(), 30*time.Second)
	return c
}
