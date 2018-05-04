package monitor

import (
	"net/http"
	"gopkg.in/mgo.v2"
)

type sourceSinkMonitor struct {

}

func (s *sourceSinkMonitor) checkResponse(r *http.Response) (failure bool, failuremsg string, err error) {
	panic("implement me")
}

type sourceSinkStoreInterface interface {
	setMaxValue(node string, maxvalue int)
	getMaxValue(node string) int
	
}

type sourceSinkStore struct {
	db *mgo.Database
}

func (sourceSinkStore) setMaxValue(node string, maxvalue int) {
	panic("implement me")
}

func (sourceSinkStore) getMaxValue(node string) int {
	panic("implement me")
}

type nodeHours struct {
	Nodename string
	BusinessHours string
	BusinessHoursImpact string
	AfterHours string
	AfterHoursImpact string
}

type nodeMax struct {
	Nodename string
	Maxval int
}

func NewMontoSourceSinkStore(db *mgo.Database) sourceSinkStoreInterface{
	return &sourceSinkStore{db:db}

}

