package sourceMonitor

import "gopkg.in/mgo.v2"

type Store interface {
	setMaxValue(node string, maxvalue int)
	getMaxValue(node string) int

	setNodeTimes(nodename, businessHours, businessCritical,afterHours, afterCticical string)
	GetNodeTimes() []nodeHours

}

type mongoStore struct {
	db *mgo.Database
}

func (s mongoStore) GetNodeTimes() []nodeHours {
	var result []nodeHours
	s.db.C("node_hours").Find(nil).All(&result)
	return result
}

func (mongoStore) setMaxValue(node string, maxvalue int) {
	panic("implement me")
}

func (mongoStore) getMaxValue(node string) int {
	panic("implement me")
}

func (s *mongoStore) setNodeTimes(nodename, businessHours, businessCritical,afterHours, afterCticical string){
	n := nodeHours{
		AfterHours:afterHours,
		AfterHoursImpact:afterCticical,
		BusinessHours:businessHours,
		BusinessHoursImpact:businessCritical,
		Nodename:nodename,
	}

	s.db.C("node_hours").Insert(&n)
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

func NewMontoSourceSinkStore(db *mgo.Database) Store{
	return &mongoStore{db:db}
}


