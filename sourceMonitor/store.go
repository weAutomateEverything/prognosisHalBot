package sourceMonitor

import (
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type Store interface {
	setNodeTimes(nodename, businessHours, businessCritical, afterHours, afterCticical string)
	GetNodeTimes() []nodeHours

	getMaxConnections() []nodeMax
	setMaxConnections([]nodeMax) error
}

type mongoStore struct {
	db *mgo.Database
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

func NewMontoSourceSinkStore(db *mgo.Database) Store {
	return &mongoStore{db: db}
}
