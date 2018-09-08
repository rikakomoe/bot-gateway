package router

import (
	. "github.com/projectriri/bot-gateway/types"
	"github.com/projectriri/bot-gateway/utils"
	log "github.com/sirupsen/logrus"
	"strings"
)

func route() {
	for {
		pkt := <-producerBuffer
		if !utils.ValidateUUID(pkt.Head.UUID) {
			log.Warnf("[router] pkt with invalid uuid, dropped: %+v BODY: %s", pkt.Head, string(pkt.Body))
			continue
		}
		log.Debugf("[router] pkt: %+v", pkt.Head)
		log.Debugf("[router] pkt: %s BODY: %s", pkt.Head.UUID, string(pkt.Body))

		for _, cc := range consumerChannelPool {
			go pushMessage(cc, &pkt)
		}
	}
}

func pushMessage(cc *ConsumerChannel, pkt *Packet) {
	log.Debugf("[router] pkt: %v TRYING cc: %+v", pkt.Head.UUID, cc)
	var formats []Format
	for _, ac := range cc.Accept {
		f := ac.FromRegexp.MatchString(pkt.Head.From)
		t := ac.ToRegexp.MatchString(pkt.Head.To)
		if f && t {
			formats = ac.Formats
			break
		}
	}

	if formats == nil {
		return
	}

	log.Debugf("[router] pkt: %v ACCEPTED BY cc: %+v", pkt.Head.UUID, cc)

	for _, format := range formats {

		if strings.ToLower(pkt.Head.Format.API) == strings.ToLower(format.API) &&
			strings.ToLower(pkt.Head.Format.Method) == strings.ToLower(format.Method) &&
			strings.ToLower(pkt.Head.Format.Protocol) == strings.ToLower(format.Protocol) {
			cc.Buffer <- *pkt
			return
		}

		for _, cvt := range converters {
			if cvt.IsConvertible(pkt.Head.Format, format) {
				ok, result := cvt.Convert(*pkt, format)
				if ok && result != nil {
					for _, p := range result {
						log.Debugf("[route] converted: %+v", string(p.Body))
						select {
						case cc.Buffer <- p:
						default:
							select {
							case <-cc.Buffer:
								log.Warnf("[router] cache overflowed, popped the oldest message of consumer buffer %v, messages in buffer: %v", cc.UUID, len(cc.Buffer))
								cc.Buffer <- p
							case cc.Buffer <- p:
							}
						}
					}
					return
				}
			}
		}
	}
}
