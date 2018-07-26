package sinkBin

import (
	"gopkg.in/mgo.v2"
)

type Store interface {
	getShardId(bin string) (int, error)
}

func NewMongoStore(db *mgo.Database) Store {
	return &mongo{
		db: db,
	}
}

type mongo struct {
	db *mgo.Database
}

func (s mongo) getShardId(bin string) (shard int, err error) {
	c := s.db.C("binshard")
	q := c.FindId(bin)
	count, err := q.Count()
	if err != nil {
		return 0, err
	}

	if count > 0 {
		var sh shardType
		err := q.One(&sh)
		return sh.Index, err
	}

	q = c.Find(nil)
	count, err = q.Count()

	if err != nil {
		return 0, err
	}

	return count, c.Insert(&shardType{
		BIN:   bin,
		Index: count,
	})

}

type shardType struct {
	BIN   string `json:"id" bson:"_id,omitempty"`
	Index int    `json:"index"`
}
