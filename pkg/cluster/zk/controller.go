package zk

import (
	"github.com/funkygao/dbus/pkg/cluster"
	"github.com/funkygao/go-zookeeper/zk"
	log "github.com/funkygao/log4go"
	"github.com/funkygao/zkclient"
)

var (
	_ cluster.Controller       = &controller{}
	_ cluster.Manager          = &controller{}
	_ zkclient.ZkStateListener = &controller{}
)

// controller is a participant in a cluster.
// It might be a leader or a normal participant that accepts RPC from the leader.
type controller struct {
	kb *keyBuilder
	zc *zkclient.Client

	strategyFunc cluster.StrategyFunc
	participant  cluster.Participant

	leader   *leader
	hc       *healthCheck
	elector  *leaderElector
	upgrader *upgrader

	// only when participant is leader will this callback be triggered.
	onRebalance cluster.RebalanceCallback
}

// New creates a Controller with zookeeper as underlying storage.
func NewController(zkSvr string, participant cluster.Participant, strategy cluster.Strategy, onRebalance cluster.RebalanceCallback) cluster.Controller {
	if onRebalance == nil {
		panic("onRebalance nil not allowed")
	}
	if len(zkSvr) == 0 {
		panic("invalid zkSvr")
	}
	if !participant.Valid() {
		panic("invalid participant")
	}
	strategyFunc := cluster.GetStrategyFunc(strategy)
	if strategyFunc == nil {
		panic("strategy not implemented")
	}

	return &controller{
		kb:           newKeyBuilder(),
		participant:  participant,
		onRebalance:  onRebalance,
		strategyFunc: strategyFunc,
		zc:           zkclient.New(zkSvr, zkclient.WithWrapErrorWithPath()),
	}
}

func (c *controller) connectToZookeeper() (err error) {
	log.Debug("[%s] connecting to zookeeper...", c.participant)
	if err = c.zc.Connect(); err != nil {
		return
	}

	for retries := 0; retries < 3; retries++ {
		if err = c.zc.WaitUntilConnected(c.zc.SessionTimeout()); err == nil {
			log.Trace("[%s] connected to zookeeper", c.participant)
			break
		}

		log.Warn("[%s] retry=%d %v", c.participant, retries, err)
	}

	return
}

func (c *controller) Start() (err error) {
	if err = c.connectToZookeeper(); err != nil {
		return
	}

	for _, path := range c.kb.persistentKeys() {
		if err = c.zc.CreateEmptyPersistentIfNotPresent(path); err != nil {
			return
		}
	}

	c.hc = newHealthCheck(c.participant, c.zc, c.kb)
	c.hc.startup()

	c.zc.SubscribeStateChanges(c)

	c.leader = newLeader(c)

	c.elector = newLeaderElector(c, c.leader.onBecomingLeader, c.leader.onResigningAsLeader)
	c.elector.startup()

	c.upgrader = newUpgrader(c)
	c.upgrader.startup()

	return
}

func (c *controller) Stop() (err error) {
	// will delete all ephemeral znodes:
	// participant, controller if leader
	c.zc.Disconnect()

	c.elector.close()
	c.hc.close()
	c.upgrader.close()

	log.Trace("[%s] controller stopped", c.participant)
	return
}

func (c *controller) Upgrade() <-chan struct{} {
	return c.upgrader.events()
}

func (c *controller) amLeader() bool {
	return c.elector.amLeader()
}

func (c *controller) HandleNewSession() (err error) {
	log.Trace("[%s] ZK expired; shutdown all controller components and try re-elect", c.participant)

	if c.amLeader() {
		c.leader.onResigningAsLeader()
	}

	c.elector.elect()
	return
}

func (c *controller) HandleStateChanged(state zk.State) (err error) {
	return
}
