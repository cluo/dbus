package kafka

import (
	"github.com/Shopify/sarama"
	log "github.com/funkygao/log4go"
)

func init() {
	sarama.PanicHandler = func(err interface{}) {
		log.Critical("sarama panic: %v", err)
	}
}
