package beanstalk

import "time"

// ManagerInterface is a manager interface for interacting with the beanstalk.
type ManagerInterface interface {
	CreateTubes(name string)
	PutJob(body []byte, pri uint32, delay, ttr time.Duration) (id uint64, err error)
	GetJob(timeout time.Duration) (id uint64, body []byte, err error)
	DeleteJob(id uint64) error
	CloseTube() error
	CloseTubeSet() error
	ReconnectTube()
	ReconnectTubeSet()
}
