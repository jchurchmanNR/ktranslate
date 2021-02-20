package metrics

import (
	"math/rand"
	"time"

	"github.com/kentik/gosnmp"
	"github.com/kentik/ktranslate/pkg/eggs/logger"
	"github.com/kentik/ktranslate/pkg/kt"
	"github.com/kentik/ktranslate/pkg/util/tick"
)

type Poller struct {
	log              logger.ContextL
	server           *gosnmp.GoSNMP
	interfaceMetrics *InterfaceMetrics
	deviceMetrics    *DeviceMetrics
	jchfChan         chan []*kt.JCHF
	metrics          *kt.SnmpDeviceMetric
}

func NewPoller(server *gosnmp.GoSNMP, conf *kt.SnmpDeviceConfig, jchfChan chan []*kt.JCHF, metrics *kt.SnmpDeviceMetric, log logger.ContextL) *Poller {

	// Default rate multiplier to 1 if its 0.
	if conf.RateMultiplier == 0 {
		conf.RateMultiplier = 1
		log.Infof("Defaulting rate multiplier to 1")
	}

	return &Poller{
		jchfChan:         jchfChan,
		log:              log,
		metrics:          metrics,
		server:           server,
		interfaceMetrics: NewInterfaceMetrics(conf, metrics, log),
		deviceMetrics:    NewDeviceMetrics(conf, metrics, log),
	}
}

func (p *Poller) StartLoop() {

	// counterChecks are a little bit tricky.  For non-snmpLow devices, we want to collect once every 5 minutes, and want to be confident that
	// we'll actually get a set of datapoints into storage for every aligned five-minute block.  That way, for every aligned chunk-size used by
	// the query system (5 minutes, 10 minutes, 1 hour) we'll have a consistent number of counter datapoints contained in each chunk.
	// Problem is, SNMP counter polls take some time, and the time varies widely from device to device, based on number of interfaces and
	// round-trip-time to the device.  So we're going to divide each aligned five minute chunk into two periods: an initial period over which
	// to jitter the devices, and the rest of the five-minute chunk to actually do the counter-polling.  For any device whose counters we can walk
	// in less than (5 minutes - jitter period), we should be able to guarantee exactly one datapoint per aligned five-minute chunk.
	counterAlignment := 1 * time.Minute // Once every 5 min. Changing from 10 to 5 to comply with internet billing standards (charged on p95 for 5 min intervals)
	jitterWindow := 15 * time.Second
	firstCollection := time.Now().Truncate(counterAlignment).Add(counterAlignment).Add(time.Duration(rand.Int63n(int64(jitterWindow))))
	counterCheck := tick.NewFixedTimer(firstCollection, counterAlignment)

	p.log.Infof("snmpCounterPoll: First poll will be at %v", firstCollection)

	go func() {
		// Track the counters here, to convert from raw counters to differences
		for scheduledTime := range counterCheck.C {

			startTime := time.Now()
			if !startTime.Truncate(counterAlignment).Equal(scheduledTime.Truncate(counterAlignment)) {
				// This poll was supposed to occur in a previous five-minute-block, but we were delayed
				// in picking it up -- presumably because a previous poll overflowed *its* block.
				// Since we can't possibly complete this one on schedule, skip it.
				p.log.Warnf("Skipping a counter datapoint for the period %v -- poll scheduled for %v, but only dequeued at %v",
					scheduledTime.Truncate(counterAlignment), scheduledTime, startTime)
				p.interfaceMetrics.DiscardDeltaState()
				continue
			}

			flows, err := p.Poll()
			if err != nil {
				p.log.Warnf("Issue polling SNMP Counter: %v", err)

				// We didn't collect all the metrics here, which means that our delta values are
				// off, and we have to discard them.
				p.interfaceMetrics.DiscardDeltaState()
				continue
			}

			// Send counter data as flow
			if !time.Now().Truncate(counterAlignment).Equal(scheduledTime.Truncate(counterAlignment)) {
				// Uggh.  calling PollSNMPCounter took us long enough that we're no longer in the five-minute block
				// we were in when we started the poll.
				p.log.Warnf("Missed a counter datapoint for the period %v -- poll scheduled for %v, started at %v, ended at %v",
					scheduledTime.Truncate(counterAlignment), scheduledTime, startTime, time.Now())

				// Because this counter poll took too long, and at least the earliest values received in the
				// poll are already over five minutes old, we can no longer use them as the basis for deltas.
				// Throw all the values away, and start over with the next polling cycle
				p.interfaceMetrics.DiscardDeltaState()
				continue
			}

			// Great!  We finished the poll in the same five-minute block we started it in!
			// send the results to Sinks.
			p.jchfChan <- flows
		}
	}()
}

// PollSNMPCounter polls SNMP for counter statistics like # bytes and packets transferred.
func (p *Poller) Poll() ([]*kt.JCHF, error) {

	deviceFlows, err := p.deviceMetrics.Poll(p.server)

	flows, err := p.interfaceMetrics.Poll(p.server)
	if err != nil {
		return nil, err
	}

	// Marshal device metrics data into flow and append them to the list.
	flows = append(flows, deviceFlows...)

	return flows, nil
}