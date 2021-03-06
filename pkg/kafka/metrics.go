package kafka

import (
	"strings"

	"github.com/funkygao/gafka/telemetry"
	"github.com/funkygao/go-metrics"
)

type producerMetrics struct {
	name string
	tag  string

	asyncSend metrics.Meter
	asyncOk   metrics.Meter
	asyncFail metrics.Meter

	syncOk   metrics.Meter
	syncFail metrics.Meter
}

func newMetrics(name string) *producerMetrics {
	tag := telemetry.Tag(strings.Replace(name, ".", "_", -1), "", "")
	return &producerMetrics{
		name:      name,
		tag:       tag,
		asyncSend: metrics.NewRegisteredMeter(tag+"dbus.kafka.async.send", metrics.DefaultRegistry),
		asyncOk:   metrics.NewRegisteredMeter(tag+"dbus.kafka.async.ok", metrics.DefaultRegistry),
		asyncFail: metrics.NewRegisteredMeter(tag+"dbus.kafka.async.fail", metrics.DefaultRegistry),

		//syncOk:    metrics.NewRegisteredMeter(tag+"dbus.kafka.sync.ok", metrics.DefaultRegistry),
		//syncFail:  metrics.NewRegisteredMeter(tag+"dbus.kafka.sync.fail", metrics.DefaultRegistry),
	}
}

func (m *producerMetrics) Close() {
	// TODO flush metrics
	metrics.Unregister(m.tag + "dbus.kafka.async.send")
	metrics.Unregister(m.tag + "dbus.kafka.async.ok")
	metrics.Unregister(m.tag + "dbus.kafka.async.fail")

	//metrics.Unregister(m.tag + "dbus.kafka.sync.ok")
	//metrics.Unregister(m.tag + "dbus.kafka.sync.fail")
}
