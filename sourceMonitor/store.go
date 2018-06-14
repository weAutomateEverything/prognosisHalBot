package sourceMonitor

import (
	"fmt"
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

	saveConnectionCount(name string, value int64) error
	getConnectionCount(name string) (int64, error)
}

type mongoStore struct {
	db *mgo.Database
}

func (s mongoStore) getConnectionCount(name string) (avg int64, err error) {
	c := s.db.C("connection_count")
	key := fmt.Sprintf("%v%v-%v", time.Now().Hour(), time.Now().Minute(), name)
	q := c.FindId(key)
	count, err := q.Count()
	if err != nil {
		return
	}

	if count == 0 {
		err = fmt.Errorf("no data found for %v", key)
		return
	}

	d := connectionCount{}
	err = q.One(&d)
	if err != nil {
		return
	}

	return d.Avg, nil

}

func (s mongoStore) saveConnectionCount(name string, value int64) error {
	c := s.db.C("connection_count")
	key := fmt.Sprintf("%v%v-%v", time.Now().Hour(), time.Now().Minute(), name)
	q := c.FindId(key)
	count, err := q.Count()
	if err != nil {
		return err
	}

	if count == 0 {
		d := &connectionCount{
			Time:  key,
			Count: 1,
			Avg:   value,
		}

		return c.Insert(d)
	}

	var d connectionCount
	err = q.One(&d)
	if err != nil {
		return err
	}

	d.Avg = int64(((d.Avg * d.Count) + value) / (d.Count + 1))
	d.Count++
	return c.UpdateId(key, d)

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
	Time       string `json:"id" bson:"_id,omitempty"`
	Connection string
	Avg        int64
	Count      int64
}
