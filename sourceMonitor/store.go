package sourceMonitor

import (
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
}

type mongoStore struct {
	db *mgo.Database
}

func (s mongoStore) saveConnectionCount(name string, value int64) error {
	c := s.db.C("connection_count")
	k := connectionCount{
		Count:      value,
		Connection: name,
		Date:       time.Now(),
	}
	return c.Insert(k)

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
	Date       time.Time
	Connection string
	Count      int64
}
