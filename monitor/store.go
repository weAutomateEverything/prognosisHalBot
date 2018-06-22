package monitor

import (
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"time"
)

type store struct {
	db *mgo.Database
}

type Store interface {
	SaveRateData(d data)
	SaveResponceCodeData([]string)
	GetCount(id string) (int, time.Time, error)
	IncreaseCount(id string) error
	ZeroCount(id string) error
}

func NewMongoStore(db *mgo.Database) Store {
	return &store{
		db: db,
	}
}

func (s *store) GetCount(id string) (int, time.Time, error) {
	c := s.db.C("failurecound")
	var r failurecount
	err := c.Find(bson.M{"apistring": id}).One(&r)
	return r.Count, r.FirstError, err
}

func (s *store) IncreaseCount(id string) error {
	c := s.db.C("failurecound")
	var r failurecount
	q := c.Find(bson.M{"apistring": id})
	count, err := q.Count()
	if err != nil {
		return err
	}

	if count == 0 {
		r.Apistring = id
		r.Count = 1
		r.FirstError = time.Now()
		return c.Insert(&r)
	} else {
		err := q.One(&r)
		if err != nil {
			return err
		}
		r.Count++
		if r.Count == 1 {
			r.FirstError = time.Now()
		}
		return c.Update(bson.M{"apistring": id}, &r)
	}
}

func (s *store) ZeroCount(id string) error {
	c := s.db.C("failurecound")
	var r failurecount
	q := c.Find(bson.M{"apistring": id})
	count, err := q.Count()
	if err != nil {
		return err
	}

	if count == 0 {
		r.Apistring = id
		r.Count = 0
		return c.Insert(&r)
	} else {
		err := q.One(&r)
		if err != nil {
			return err
		}
		r.Count = 0
		return c.Update(bson.M{"apistring": id}, &r)
	}
}

func (s *store) SaveRateData(d data) {
	c := s.db.C("ratedata")
	r := rateRecord{
		Date:     time.Now(),
		Approved: d.approved,
		Declined: d.declined,
		Failed:   d.failed,
	}
	c.Insert(&r)
}

func (s *store) SaveResponceCodeData(d []string) {
	c := s.db.C("responsecode")
	r := responceCodeRecord{
		Date:          time.Now(),
		ResponseCodes: d,
	}

	c.Insert(&r)
}

type rateRecord struct {
	Date                       time.Time
	Failed, Approved, Declined int
}

type responceCodeRecord struct {
	Date          time.Time
	ResponseCodes []string
}

type failurecount struct {
	ID         bson.ObjectId `bson:"_id,omitempty"`
	Apistring  string
	Count      int
	FirstError time.Time
}
