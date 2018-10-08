package monitor

import (
	"gopkg.in/mgo.v2"
	"time"
)

type Store interface {
	SaveRateData(d data)
	SaveResponceCodeData([]string)
	GetCount(id string, key string) (int, time.Time, error)
	IncreaseCount(id string, key string) error
	ZeroCount(id string, key string) error
	SetMessageSent(id string, key string) error
	SetCalloutInvoked(is string, key string) error
	IsMessageSent(id string, key string) (bool, error)
	IsCalloutInvoked(id string, key string) (bool, error)
}

func NewMongoStore(db *mgo.Database) Store {
	return &store{
		db: db,
	}
}

type store struct {
	db *mgo.Database
}

func (s *store) IsMessageSent(id string, key string) (bool, error) {
	c := s.db.C("failurecound")
	var r failurecount
	err := c.FindId(id + key).One(&r)

	if err != nil {
		return false, err
	}
	return r.MessageSent, nil
}

func (s *store) IsCalloutInvoked(id string, key string) (bool, error) {
	c := s.db.C("failurecound")
	var r failurecount
	err := c.FindId(id + key).One(&r)

	if err != nil {
		return false, err
	}
	return r.CalloutInvoked, nil
}

func (s *store) SetMessageSent(id string, key string) error {
	c := s.db.C("failurecound")
	var r failurecount
	err := c.FindId(id + key).One(&r)

	if err != nil {
		return err
	}

	r.MessageSent = true

	return c.UpdateId(id+key, &r)

}

func (s *store) SetCalloutInvoked(id string, key string) error {
	c := s.db.C("failurecound")
	var r failurecount
	err := c.FindId(id + key).One(&r)

	if err != nil {
		return err
	}

	r.CalloutInvoked = true

	return c.UpdateId(id+key, &r)
}

func (s *store) GetCount(id string, key string) (int, time.Time, error) {
	c := s.db.C("failurecound")
	var r failurecount
	err := c.FindId(id + key).One(&r)
	return r.Count, r.FirstError, err
}

func (s *store) IncreaseCount(id string, key string) error {
	c := s.db.C("failurecound")
	var r failurecount
	q := c.FindId(id + key)
	count, err := q.Count()
	if err != nil {
		return err
	}

	if count == 0 {
		r.ID = id + key
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
		return c.UpdateId(id+key, &r)
	}
}

func (s *store) ZeroCount(id string, key string) error {
	c := s.db.C("failurecound")
	var r failurecount
	q := c.FindId(id + key)
	count, err := q.Count()
	if err != nil {
		return err
	}

	if count == 0 {
		r.ID = id + key
		r.Count = 0
		return c.Insert(&r)
	} else {
		err := q.One(&r)
		if err != nil {
			return err
		}
		r.Count = 0
		r.MessageSent = false
		r.CalloutInvoked = false
		return c.UpdateId(id+key, &r)
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
	ID             string `bson:"_id,omitempty"`
	Count          int
	FirstError     time.Time
	MessageSent    bool
	CalloutInvoked bool
}
