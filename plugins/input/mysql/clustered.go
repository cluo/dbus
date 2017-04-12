package mysql

import (
	"sync"
	"time"

	"github.com/funkygao/dbus/engine"
	"github.com/funkygao/dbus/pkg/cluster"
	"github.com/funkygao/dbus/pkg/myslave"
	log "github.com/funkygao/log4go"
)

func (this *MysqlbinlogInput) runClustered(r engine.InputRunner, h engine.PluginHelper) error {
	defer func() {
		close(this.stopped)
	}()

	this.stopped = make(chan struct{})
	name := r.Name()
	backoff := time.Second * 5
	ex := r.Exchange()

	globals := engine.Globals()
	var myResources []cluster.Resource
	resourcesCh := r.Resources()

	reapSlaves := func(wg *sync.WaitGroup, stopper chan<- struct{}) {
		close(stopper)
		wg.Wait()
	}

	for {
	RESTART_REPLICATION:

		// wait till got some resource
		for {
			if len(myResources) != 0 {
				log.Trace("[%s] bingo! %d: %+v", name, len(myResources), myResources)
				break
			}

			log.Trace("[%s] awaiting resources", name)

			select {
			case <-this.stopChan:
				log.Debug("[%s] yes sir!", name)
				return nil
			case myResources = <-resourcesCh:
			}
		}

		var wg sync.WaitGroup
		slavesStopper := make(chan struct{})
		replicationErrs := make(chan error)

		// got new resources assignment!

		this.mu.Lock()
		this.slaves = this.slaves[:0]
		for _, resource := range myResources {
			dsn := resource.DSN()
			theSlave := myslave.New(name, dsn, globals.ZrootCheckpoint).LoadConfig(this.cf)
			this.slaves = append(this.slaves, theSlave)

			wg.Add(1)
			go this.runSlaveReplication(theSlave, name, ex, &wg, slavesStopper, replicationErrs)
		}
		this.mu.Unlock()

		for {
			select {
			case <-this.stopChan:
				reapSlaves(&wg, slavesStopper)
				return nil

			case myResources = <-resourcesCh:
				log.Trace("[%s] cluster rebalanced, restart replication", name)
				reapSlaves(&wg, slavesStopper)
				goto RESTART_REPLICATION

			case <-replicationErrs:
				// e,g.
				// ERROR 1236 (HY000): Could not find first log file name in binary log index file
				// ERROR 1236 (HY000): Could not open log file
				// read initial handshake error, caused by Too many connections

				// myResources not changed, so next round still consume the same resources

				select {
				case <-time.After(backoff):
				case <-this.stopChan:
					reapSlaves(&wg, slavesStopper)
					return nil
				}
				goto RESTART_REPLICATION
			}
		}
	}

	return nil
}

func (this *MysqlbinlogInput) runSlaveReplication(slave *myslave.MySlave, name string, ex engine.Exchange, wg *sync.WaitGroup,
	slavesStopper <-chan struct{}, replicationErrs chan<- error) {
	defer func() {
		log.Trace("[%s] stopping replication from %s", name, slave.DSN())

		slave.StopReplication()
		wg.Done()
	}()

	log.Trace("[%s] starting replication from %s", name, slave.DSN())
	if err := slave.AssertValidRowFormat(); err != nil {
		// err might be: read initial handshake error
		panic(err)
	}

	if img, err := slave.BinlogRowImage(); err != nil {
		log.Error("[%s] %v", name, err)
	} else {
		log.Trace("[%s] binlog row image=%s", name, img)
	}

	ready := make(chan struct{})
	go slave.StartReplication(ready)
	select {
	case <-ready:
	case <-slavesStopper:
		return
	}

	rows := slave.Events()
	errors := slave.Errors()
	for {
		select {
		case <-slavesStopper:
			return

		case err, ok := <-errors:
			if ok {
				log.Error("[%s] %v, stop from %s", name, err, slave.DSN())
				replicationErrs <- err
			}
			return

		case pack, ok := <-ex.InChan():
			if !ok {
				return
			}

			select {
			case <-slavesStopper:
				return

			case err, ok := <-errors:
				if ok {
					replicationErrs <- err
				}
				return

			case row, ok := <-rows:
				if !ok {
					log.Info("[%s] event stream closed from %s", name, slave.DSN())
					return
				}

				if row.Length() < this.maxEventLength {
					pack.Payload = row
					ex.Inject(pack)
				} else {
					// TODO this.slave.MarkAsProcessed(r), also consider batcher partial failure
					log.Warn("[%s] ignored len=%d %s", name, row.Length(), row.MetaInfo())
					pack.Recycle()
				}

			}
		}
	}
}
