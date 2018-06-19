package sourceMonitor

import (
	"github.com/pkg/errors"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"time"
)

func NewMontoSourceSinkStore(db *mgo.Database) Store {
	return &mongoStore{db: db}
}

type Store interface {
	setNodeTimes(nodename, businessHours, businessCritical, afterHours, afterCticical string)
	GetNodeTimes() []nodeHours

	getMaxConnections() []nodeMax
	setMaxConnections([]nodeMax) error

	saveConnectionCount(name string, value int) error
	getConnectionCount(name string) (float64, error)
}

type mongoStore struct {
	db *mgo.Database
}

func (s mongoStore) getConnectionCount(name string) (avg float64, err error) {
	c := s.db.C("connection_data")
	d := time.Now()
	q := []bson.M{
		{
			"$match": bson.M{
				"connection": bson.M{"$eq": name},
				"hour":       bson.M{"$eq": d.Hour()},
				"minute":     bson.M{"$eq": d.Minute()},
			},
		},
		{

			"$group": bson.M{
				"_id": nil,
				"average_response": bson.M{
					"$avg": "$connections",
				},
			},
		},
	}

	var r []bson.M
	err = c.Pipe(q).All(&r)
	if err != nil {
		return
	}

	avg, ok := r[0]["average_response"].(float64)
	if !ok {
		err = errors.New("No data found")
	}
	return
}

func (s mongoStore) saveConnectionCount(name string, value int) error {
	c := s.db.C("connection_data")
	t := time.Now()
	q := connectionCount{
		Minute:      t.Minute(),
		Hour:        t.Hour(),
		Connection:  name,
		Connections: value,
		Day:         t.Day(),
		DayOfWeek:   int(t.Weekday()),
		Month:       int(t.Month()),
	}
	return c.Insert(&q)
}

func (s mongoStore) getMaxConnections() (result []nodeMax) {
	c := s.db.C("max_connections")
	c.Find(nil).All(&result)
	return
}

func (s mongoStore) setMaxConnections(req []nodeMax) (err error) {
	c := s.db.C("max_connections")
	_, err = c.RemoveAll(bson.M{"id": "*"})
	if err != nil {
		return
	}

	for _, r := range req {
		err = c.Insert(r)
		if err != nil {
			return
		}
	}
	return

}

func (s mongoStore) GetNodeTimes() []nodeHours {
	var result []nodeHours
	s.db.C("node_hours").Find(nil).All(&result)
	return result
}

func (s *mongoStore) setNodeTimes(nodename, businessHours, businessCritical, afterHours, afterCticical string) {
	n := nodeHours{
		AfterHours:          afterHours,
		AfterHoursImpact:    afterCticical,
		BusinessHours:       businessHours,
		BusinessHoursImpact: businessCritical,
		Nodename:            nodename,
	}

	s.db.C("node_hours").Insert(&n)
}

type nodeHours struct {
	Nodename            string
	BusinessHours       string
	BusinessHoursImpact string
	AfterHours          string
	AfterHoursImpact    string
}

type nodeMax struct {
	Nodename string
	Maxval   int
}

type connectionCount struct {
	Connection  string `json:"connection"`
	Hour        int    `json:"hour"`
	Minute      int    `json:"minute"`
	DayOfWeek   int    `json:"day_of_week"`
	Day         int    `json:"day"`
	Month       int    `json:"month"`
	Connections int    `json:"connections"`
}
