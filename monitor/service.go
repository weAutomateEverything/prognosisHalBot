package monitor

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/antchfx/htmlquery"
	"github.com/kyokomi/emoji"
	"github.com/weAutomateEverything/go2hal/callout"
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
	CheckResponse(ctx context.Context, s [][]string) (response []Response, err error)
	GetName() string
}

type Service interface {
}

type Response struct {
	Key        string
	Failure    bool
	FailureMsg string
}

type service struct {
	store Store

	monitors map[string]Monitor

	failing    bool
	cookie     []*http.Cookie
	config     environment
	currentEnv int

	techErrCount int
}

func NewService(store Store, monitors ...Monitor) Service {
	s := service{
		store: store,
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

	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
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
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	http.DefaultClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	ctx := context.Background()
	//Login - get the cookie for auth
	s.getLoginCookie(ctx)

	for true {
		s.checkPrognosis()
	}
}

func (s *service) checkPrognosis() {
	log.Println("Starting Prognosis")

	ctx := context.Background()

	for _, monitor := range s.config.Monitors {
		response, err := s.checkMonitor(ctx, monitor)

		//If there is an error fetching data, lets handle it, but not use the results to determine the system health
		if err != nil {
			s.techErrCount++
			log.Printf("Tech Error count is %v", s.techErrCount)
			if s.techErrCount == 10 {
				s.sendMessage(ctx, "10 failures detected. Attempting login to find a new host", getErrorGroup())
				panic("10 consecutive failures")
			}
			continue
		}
		log.Println("setting tech error count to 0")
		s.techErrCount = 0

		for _, resp := range response {
			if resp.Failure {
				s.handleFailed(ctx, monitor, resp)
			} else {
				_, t, err := s.store.GetCount(monitor.Name, resp.Key)
				if err != nil {
					continue
				}
				d := time.Since(t).Truncate(time.Second)
				sent, err := s.store.IsMessageSent(monitor.Name, resp.Key)
				if err != nil {
					s.sendMessage(ctx, fmt.Sprintf("Error checking if a message has been sent. %v", err.Error()), getErrorGroup())
					continue
				}
				if sent {
					s.sendMessage(ctx, emoji.Sprintf(":white_check_mark: No issues detected for %v %v. Errors occurred for %v", monitor.Name, resp.Key, d.String()), monitor.Group)
				}
				s.store.ZeroCount(monitor.Name, resp.Key)
			}
		}
	}
}

func (s *service) handleFailed(ctx context.Context, monitor *monitors, response Response) {
	err := s.store.IncreaseCount(monitor.Name, response.Key)
	if err != nil {
		log.Println(err)
		s.sendMessage(ctx, err.Error(), getErrorGroup())
		return
	}
	_, t, err := s.store.GetCount(monitor.Name, response.Key)
	d := time.Since(t).Truncate(time.Second)

	if err != nil {
		log.Println(err)
		s.sendMessage(ctx, err.Error(), getErrorGroup())
		return
	}
	//Ignore the first 2 errors - this should make the alerts less noisy
	if d > 30*time.Second {
		log.Printf("Sendign warning for %v", monitor.Name)
		s.sendMessage(ctx, emoji.Sprintf(":x: %v. Error has been occurring for %v.", response.FailureMsg, d.String()), monitor.Group)
		s.store.SetMessageSent(monitor.Name, response.Key)
	}

	//After 15 alerts, lets invoke callout
	if d > 3*time.Minute {
		calloutInvoked, err := s.store.IsCalloutInvoked(monitor.Name, response.Key)
		if err != nil {
			s.sendMessage(ctx, fmt.Sprintf("Error checking if callout has been invoked: %v", err.Error()), getErrorGroup())
			return

		}
		if !calloutInvoked {
			log.Printf("Invoking callout for %v %v\n", monitor.Name, response.Key)

			c := callout.SendCalloutRequest{
				Message: fmt.Sprintf("Prognosis Issue Detected. %v", response.FailureMsg),
				Title:   response.FailureMsg,
			}

			b, err := json.Marshal(c)
			if err != nil {
				return
			}
			resp, err := http.Post(fmt.Sprintf("%v/api/callout/%v", os.Getenv("HAL_ENDPOINT"), monitor.Group),
				"application/json", bytes.NewReader(b))
			if err != nil {
				return
			}
			resp.Body.Close()
			err = s.store.SetCalloutInvoked(monitor.Name, response.Key)
			if err != nil {
				s.sendMessage(ctx, fmt.Sprintf("Error setting callout invoked: %v", err.Error()), getErrorGroup())
			}
		}
	}
	return
}

func (s *service) getLoginCookie(ctx context.Context) (err error) {
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
				s.sendMessage(ctx, "prognosis error - "+err.Error(), getErrorGroup())
				continue
			}
			if len(resp.Cookies()) == 0 {
				s.sendMessage(ctx, "Prognosis error - No cookie found on response", getErrorGroup())
				continue
			}
			s.currentEnv = i
			defer resp.Body.Close()
			s.cookie = resp.Cookies()
			return nil
		}
		s.sendMessage(ctx, "Unable to successfully log into prognosis... will try again in 60 seconds", getErrorGroup())
		time.Sleep(60 * time.Second)
	}
	return nil

}

func (s *service) checkMonitor(ctx context.Context, monitor *monitors) (response []Response, err error) {
	count := 0
	for count < 10 {
		if count > 0 {
			time.Sleep(1 * time.Second)
		}
		count++
		guid, err := s.getGuidForMonitor(ctx, monitor)
		if err != nil {
			log.Println(err)
			continue
		}
		if monitor.ObjectType == "" {
			monitor.ObjectType = "#"
		}
		url := fmt.Sprintf("%v/Prognosis/DashboardView/%v",
			s.getEndpoint(),
			guid,
		)
		req, err := http.NewRequest("GET", url, strings.NewReader(""))
		if err != nil {
			continue
		}
		for _, c := range s.cookie {
			req.AddCookie(c)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Println(err)
			continue
		}
		var data map[string]interface{}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Println(err)
			continue
		}

		log.Println(string(body))

		err = json.Unmarshal(body, &data)

		resp.Body.Close()
		if err != nil {
			log.Println(err)
			continue
		}

		if data == nil {
			err = fmt.Errorf("data nil for dashboard %v, id %v", monitor.Dashboard, guid)
			log.Println(err)
			continue
		}

		//First the root element - this should include a item called Data
		key := monitor.Id
		if strings.HasPrefix(key, "id_") {
			key = strings.Replace(key, "id_", "", 1)
		}
		root, ok := data[key]
		if !ok {
			log.Println("No valid root element found")
			continue
		}
		rootMap := root.(map[string]interface{})

		dataObject, ok := rootMap["Data"]
		if !ok {
			log.Println("No Data object found")
			continue
		}

		dataArray := (dataObject).([]interface{})

		if len(dataArray) == 0 {
			//Sometimes, it takes prognosis a while to wake up... so the first 10 no data we can ignore
			log.Printf("Length of data is 0 for dashboard %v, graph %v", monitor.Dashboard, monitor.Name)
			continue
		}
		dataElements := dataArray[0].([]interface{})
		if len(dataElements) == 2 {
			continue
		}
		var input [][]string
		for _, row := range dataElements[2:] {
			var val []string
			for _, m := range row.([]interface{}) {
				val = append(val, m.(string))
			}
			input = append(input, val)
		}
		monitor.lastSuccess = time.Now().UnixNano()
		monitor := s.monitors[monitor.Type]
		log.Printf(monitor.GetName())
		return monitor.CheckResponse(ctx, input)

	}
	s.sendMessage(ctx, fmt.Sprintf("No data found after 10 attempts for dashboard %v", monitor.Name), getErrorGroup())
	err = NoResultsError{Messsage: fmt.Sprintf("no data found for %v, graph %v", monitor.Dashboard, getErrorGroup())}
	return nil, err

}

func (s *service) getGuidForMonitor(ctx context.Context, monitor *monitors) (guid string, err error) {
	url := fmt.Sprintf("%v/Prognosis/Dashboard/Content/%v", s.getEndpoint(), monitor.Dashboard)
	req, err := http.NewRequest("GET", url, strings.NewReader(""))
	for _, c := range s.cookie {
		req.AddCookie(c)
	}
	resp, err := http.DefaultClient.Do(req)
	defer resp.Body.Close()

	doc, err := htmlquery.Parse(resp.Body)

	if err != nil {
		return "", nil
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
	msg := fmt.Sprintf("no guid found for %v on dashboard %v. Restarting Bot", monitor.Name, getErrorGroup())
	s.sendMessage(ctx, msg, getErrorGroup())
	panic(msg)
	return

}

func (s *service) sendMessage(ctx context.Context, message string, group int64) {
	message = strings.Replace(message, "_", " ", -1)

	resp, err := http.Post(fmt.Sprintf("%v/api/alert/%v", os.Getenv("HAL_ENDPOINT"), group),
		"application/text", strings.NewReader(message))
	if err != nil {
		log.Printf("error sending message %v to group %v - error %v", message, group, err)
		return
	}

	resp.Body.Close()

}

type httpLogger struct {
	log *log.Logger
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
	Monitors []*monitors
}

type monitors struct {
	Type, Dashboard, Id, Name, ObjectType string
	Group                                 int64
	lastSuccess                           int64
}

func getTimeout() context.Context {
	c, _ := context.WithTimeout(context.TODO(), 30*time.Second)
	return c
}
