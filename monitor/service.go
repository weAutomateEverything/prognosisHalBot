package monitor

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/antchfx/htmlquery"
	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/ernesto-jimenez/httplogger"
	"github.com/kyokomi/emoji"
	"github.com/pkg/errors"
	"github.com/weAutomateEverything/go2hal/callout"
	"golang.org/x/net/context"
	"golang.org/x/net/context/ctxhttp"
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
	CheckResponse(ctx context.Context, s [][]string) (failure bool, failuremsg string, err error)
	GetName() string
}

type Service interface {
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
	ctx, seg := xray.BeginSegment(context.Background(), "Prognosis Login")
	s.getLoginCookie(ctx)
	seg.Close(nil)

	for true {
		s.checkPrognosis()
	}
}

func (s *service) checkPrognosis() {
	log.Println("Starting Prognosis")
	ctx, seg := xray.BeginSegment(context.Background(), "Prognosis Check")
	defer seg.Close(nil)

	for _, monitor := range s.config.Monitors {
		ctx, subseg := xray.BeginSubsegment(ctx, monitor.Name)
		failed, failmsg, err := s.checkMonitor(ctx, monitor)
		log.Printf("Output of check %v is, failed %v, failmsg: %v, error: %v", monitor.Name, failed, failmsg, err)

		//If there is an error fetching data, lets handle it, but not use the results to determine the system health
		if err != nil {
			s.techErrCount++
			log.Printf("Tech Error count is %v", s.techErrCount)
			if s.techErrCount == 10 {
				s.sendMessage(ctx, "10 failures detected. Attempting login to find a new host", getErrorGroup())
				xray.AddError(ctx, errors.New("10 failures detected. Attempting login to find a new host"))
				panic("10 consecutive failures")
			}
			continue
		} else {
			log.Println("setting tech error count to 0")
			s.techErrCount = 0
		}

		if failed {
			s.handleFailed(ctx, monitor, failmsg)
		} else {
			_, t, _ := s.store.GetCount(monitor.Id)
			d := time.Since(t)
			if monitor.messageSent {
				s.sendMessage(ctx, emoji.Sprintf(":white_check_mark: No issues detected. Errors occurred for %v", d.String()), monitor.Group)
			}
			s.store.ZeroCount(monitor.Id)
			monitor.calloutInvoked = false
			monitor.messageSent = false
		}
		subseg.Close(nil)
	}
}

func (s *service) handleFailed(ctx context.Context, monitor *monitors, failmsg string) {
	log.Printf("handling failure %v from %v\n", failmsg, monitor.Name)
	err := s.store.IncreaseCount(monitor.Id)
	if err != nil {
		log.Println(err)
		s.sendMessage(ctx, err.Error(), getErrorGroup())
	}
	count, t, err := s.store.GetCount(monitor.Id)
	d := time.Since(t)
	log.Printf("errror count is %v for %v\n", count, monitor.Name)

	if err != nil {
		log.Println(err)
		s.sendMessage(ctx, err.Error(), monitor.Group)
	}
	//Ignore the first 2 errors - this should make the alerts less noisy
	if d > 30*time.Second {
		log.Printf("Sendign warning for %v", monitor.Name)
		s.sendMessage(ctx, emoji.Sprintf(":x: %v. Error has been occurring for %v.", failmsg, d.String()), monitor.Group)
		monitor.messageSent = true
	}

	//After 15 alerts, lets invoke callout
	if d > 3*time.Minute {
		if !monitor.calloutInvoked {
			log.Printf("Invoking callout for %v\n", monitor.Name)

			c := callout.SendCalloutRequest{
				Message: fmt.Sprintf("Prognosis Issue Detected. %v", failmsg),
				Title:   failmsg,
			}

			b, err := json.Marshal(c)
			if err != nil {
				xray.AddError(ctx, err)
				return
			}
			resp, err := ctxhttp.Post(ctx, xray.Client(nil), fmt.Sprintf("%v/api/callout/%v", os.Getenv("HAL_ENDPOINT"), monitor.Group),
				"application/json", bytes.NewReader(b))
			if err != nil {
				xray.AddError(ctx, err)
				return
			}
			resp.Body.Close()
			monitor.calloutInvoked = true
		}
	}
	return
}

func (s *service) getLoginCookie(ctx context.Context) (err error) {
	ctx, subseg := xray.BeginSubsegment(ctx, "login")
	defer subseg.Close(err)
	for true {
		for i, x := range s.config.Address {
			log.Println("Logging in")
			v := url.Values{}
			v.Add("UserName", getUsername())
			v.Add("Password", getPassword())
			v.Add("Destination", "View Systems")
			req, err := http.NewRequest("POST", fmt.Sprintf("%v/Prognosis/Login?returnUrl=/Prognosis/", x), strings.NewReader(v.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			resp, err := ctxhttp.Do(ctx, http.DefaultClient, req)

			if err != nil {
				s.sendMessage(ctx, "prognosis error - "+err.Error(), getErrorGroup())
				xray.AddError(ctx, err)
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

func (s *service) checkMonitor(ctx context.Context, monitor *monitors) (failing bool, message string, err error) {
	ctx, subseg := xray.BeginSubsegment(ctx, monitor.Name)
	defer subseg.Close(err)
	count := 0
	for count < 10 {
		if count > 0 {
			time.Sleep(1 * time.Second)
		}
		count++
		guid, err := s.getGuidForMonitor(ctx, monitor)
		if err != nil {
			log.Println(err)
			xray.AddError(ctx, err)
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
			xray.AddError(ctx, err)
			continue
		}
		for _, c := range s.cookie {
			req.AddCookie(c)
		}

		resp, err := ctxhttp.Do(ctx, http.DefaultClient, req)
		if err != nil {
			log.Println(err)
			xray.AddError(ctx, err)
			continue
		}
		var data map[string]interface{}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Println(err)
			xray.AddError(ctx, err)
			continue
		}

		log.Println(string(body))

		err = json.Unmarshal(body, &data)

		resp.Body.Close()
		if err != nil {
			log.Println(err)
			xray.AddError(ctx, err)
			continue
		}

		if data == nil {
			err = fmt.Errorf("data nil for dashboard %v, id %v", monitor.Dashboard, guid)
			log.Println(err)
			xray.AddError(ctx, err)
			continue
		}

		//First the root element - thios should include a item called Data
		key := monitor.Id
		if strings.HasPrefix(key, "id_") {
			key = strings.Replace(key, "id_", "", 1)
		}
		root, ok := data[key]
		if !ok {
			log.Println("No valid root element found")
			xray.AddError(ctx, errors.New("No valid root element found"))
			continue
		}
		rootMap := root.(map[string]interface{})

		dataObject, ok := rootMap["Data"]
		if !ok {
			log.Println("No Data object found")
			xray.AddError(ctx, errors.New("No Data object found"))

			continue
		}

		dataArray := (dataObject).([]interface{})

		if len(dataArray) == 0 {
			//Sometimes, it takes prognosis a while to wake up... so the first 10 no data we can ignore
			log.Printf("Length of data is 0 for dashboard %v, graph %v", monitor.Dashboard, monitor.Id)
			xray.AddError(ctx, fmt.Errorf("length of data is 0 for dashboard %v, graph %v", monitor.Dashboard, monitor.Id))
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
	s.sendMessage(ctx, fmt.Sprintf("No data found after 10 attempts for dashboard %v", monitor.Id), getErrorGroup())
	err = NoResultsError{Messsage: fmt.Sprintf("no data found for %v, graph %v", monitor.Dashboard, getErrorGroup())}
	xray.AddError(ctx, err)
	return false, "", err

}

func (s *service) getGuidForMonitor(ctx context.Context, monitor *monitors) (guid string, err error) {
	ctx, subseg := xray.BeginSubsegment(ctx, "Get Guid")
	defer subseg.Close(err)
	url := fmt.Sprintf("%v/Prognosis/Dashboard/Content/%v", s.getEndpoint(), monitor.Dashboard)
	req, err := http.NewRequest("GET", url, strings.NewReader(""))
	for _, c := range s.cookie {
		req.AddCookie(c)
	}
	resp, err := ctxhttp.Do(ctx, xray.Client(nil), req)
	defer resp.Body.Close()

	doc, err := htmlquery.Parse(resp.Body)

	if err != nil {
		xray.AddError(ctx, err)
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
	msg := fmt.Sprintf("no guid found for %v on dashboard %v. Restarting Bot", monitor.Id, getErrorGroup())
	s.sendMessage(ctx, msg, getErrorGroup())
	xray.AddError(ctx, fmt.Errorf("no guid found for %v on dashboard %v. Restarting Bot", monitor.Id, getErrorGroup()))
	panic(msg)
	return

}

func (s *service) sendMessage(ctx context.Context, message string, group int64) {
	message = strings.Replace(message, "_", " ", -1)

	resp, err := ctxhttp.Post(ctx, xray.Client(nil), fmt.Sprintf("%v/api/alert/%v", os.Getenv("HAL_ENDPOINT"), group),
		"application/text", strings.NewReader(message))
	if err != nil {
		xray.AddError(ctx, err)
		return
	}

	resp.Body.Close()


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
	Monitors []*monitors
}

type monitors struct {
	Type, Dashboard, Id, Name, ObjectType string
	Group                                 int64
	lastSuccess                           int64
	calloutInvoked                        bool
	messageSent                           bool
}

func getTimeout() context.Context {
	c, _ := context.WithTimeout(context.TODO(), 30*time.Second)
	return c
}
