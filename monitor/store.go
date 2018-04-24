package monitor

import (
	"gopkg.in/mgo.v2"
	"time"
	"gopkg.in/mgo.v2/bson"
)

type store struct {
	db *mgo.Database
}

type Store interface {
	saveRateData(d data)
	saveResponceCodeData([]string)
	getCount(id string) (int, error)
	increaseCount(id string) error
	zeroCount(id string) error
}

func NewMongoStore(db *mgo.Database) Store {
	return &store{
		db:db,
	}
}

func (s *store) getCount(id string) (int,error){
	c := s.db.C("failurecound")
	var r failurecount
	err := c.Find(bson.M{"apistring":id}).One(&r)
	return r.Count, err
}

func (s *store) increaseCount(id string) error {
	c := s.db.C("failurecound")
	var r failurecount
	err := c.Find(bson.M{"apistring": id}).One(&r)
	if err != nil {
		return err
	}
	r.Count++
	return c.Insert(&r)
}


func (s *store) zeroCount(id string) error {
	c := s.db.C("failurecound")
	var r failurecount
	err := c.Find(bson.M{"apistring": id}).One(&r)
	if err != nil {
		return err
	}
	r.Count = 0
	return c.Insert(&r)
}
func (s *store) saveRateData(d data) {
	c := s.db.C("ratedata")
	r := rateRecord{
		Date:time.Now(),
		Approved:d.approved,
		Declined:d.declined,
		Failed:d.failed,
	}
	c.Insert(&r)
}

func (s *store) saveResponceCodeData(d []string) {
	c := s.db.C("responsecode")
	r := responceCodeRecord{
		Date:time.Now(),
		ResponseCodes: d,
	}

	c.Insert(&r)
}

type rateRecord struct {
	Date time.Time
	Failed, Approved, Declined int
}

type responceCodeRecord struct {
	Date time.Time
	ResponseCodes []string
}

type failurecount struct {
	ID           bson.ObjectId `bson:"_id,omitempty"`
	Apistring string
	Count int

}