package monitor

import (
	"gopkg.in/mgo.v2"
	"time"
)

type store struct {
	db *mgo.Database
}

type Store interface {
	saveRateData(d data)
	saveResponceCodeData([]string)
}

func NewMongoStore(db *mgo.Database) Store {
	return &store{
		db:db,
	}
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
		responseCodes: d,
	}

	c.Insert(&r)
}

type rateRecord struct {
	Date time.Time
	Failed, Approved, Declined int
}

type responceCodeRecord struct {
	Date time.Time
	responseCodes []string
}

