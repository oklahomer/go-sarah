package worker

import (
	"errors"
	"github.com/Sirupsen/logrus"
	"sync"
)

type pooledWorker struct {
	ID int
}

/*
Pool holds desired number of worker instances, dispatch jobs to them, and handle their lifecycle.
*/
type Pool struct {
	workers     []*pooledWorker
	isRunning   bool
	mutex       *sync.Mutex
	jobReceiver chan func()
	stop        chan bool
}

/*
NewPool is a helper function that construct and return new Pool instance.
*/
func NewPool(workerNum int) *Pool {
	var workers = make([]*pooledWorker, workerNum)
	for i := range workers {
		workers[i] = &pooledWorker{ID: i + 1}
	}
	return &Pool{
		workers:     workers,
		isRunning:   false,
		mutex:       &sync.Mutex{},
		stop:        make(chan bool),
		jobReceiver: make(chan func(), 100),
	}
}

/*
Run prepares all underlying workers to receive jobs for execution.
Once Run is called, this Pool receives job until Sop is called.
*/
func (pool *Pool) Run() error {
	logrus.Infof("start workers")
	pool.mutex.Lock()
	defer pool.mutex.Unlock()

	if pool.isRunning == true {
		return errors.New("workers are already running")
	}

	for _, worker := range pool.workers {
		go pool.runWorker(worker)
	}
	pool.isRunning = true

	return nil
}

func (pool *Pool) runWorker(worker *pooledWorker) {
	logrus.Infof("start worker id: %d.", worker.ID)
	for {
		select {
		case <-pool.stop:
			logrus.Infof("stopping worker id: %d", worker.ID)
			return
		case job := <-pool.jobReceiver:
			logrus.Infof("receiving job on worker: %d", worker.ID)
			// To avoid given job's panic affect later jobs, wrap them with recover.
			func() {
				defer func() {
					if r := recover(); r != nil {
						logrus.Warnf("panic in given job. recovered: %+v", r)
					}
				}()
				job()
			}()
		}
	}
}

/*
Stop lets underlying workers stop receiving jobs
*/
func (pool *Pool) Stop() error {
	logrus.Infof("stop workers")
	pool.mutex.Lock()
	defer pool.mutex.Unlock()

	if pool.isRunning != true {
		return errors.New("workers are already stopped")
	}
	close(pool.stop)
	pool.isRunning = false

	return nil
}

/*
EnqueueJob appends new job to be executed.
*/
func (pool *Pool) EnqueueJob(job func()) {
	pool.jobReceiver <- job
}
