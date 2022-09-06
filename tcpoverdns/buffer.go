package tcpoverdns

import (
	"sync"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
)

// SegmentBuffer is keeps a small backlog of segments transported in a single
// direction, and performs de-duplication and other optimisations as more
// segments arrive.
type SegmentBuffer struct {
	mutex     *sync.Mutex
	backlog   []Segment
	logger    lalog.Logger
	debug     bool
	maxSegLen int
}

// NewSegmentBuffer returns a newly initialised segment buffer.
func NewSegmentBuffer(logger lalog.Logger, debug bool, maxSegLen int) *SegmentBuffer {
	return &SegmentBuffer{
		mutex:     new(sync.Mutex),
		backlog:   make([]Segment, 0, 128),
		logger:    logger,
		debug:     debug,
		maxSegLen: maxSegLen,
	}
}

// SetParameters sets the max segment length and debug parameters.
func (buf *SegmentBuffer) SetParameters(segLen int, debug bool) {
	buf.mutex.Lock()
	defer buf.mutex.Unlock()
	buf.debug = debug
	buf.maxSegLen = segLen
	if buf.debug {
		buf.logger.Info("SetParameters", nil, nil, "new max seg len is %d", segLen)
	}
}

// Absorb places the segment into the backlog and optimises adjacent segments
// where possible.
func (buf *SegmentBuffer) Absorb(seg Segment) {
	buf.mutex.Lock()
	defer buf.mutex.Unlock()
	if seg.Flags.Has(FlagKeepAlive) {
		// Add random data to the keep-alive segment to prevent DNS caching.
		seg.Data = misc.RandomBytes(4)
	}
	var latest Segment
	if len(buf.backlog) > 0 {
		latest = buf.backlog[len(buf.backlog)-1]
	}
	if latest.Flags.Has(FlagAckOnly) || latest.Flags.Has(FlagKeepAlive) {
		// Merge adjacent ack-only and keep-alive segments. These segments
		// do not carry useful data and the newer ones are more useful than
		// the older ones.
		if buf.debug {
			buf.logger.Info("Absorb", "", nil, "removing the older ack/keepalive segment: %+v", buf.backlog[len(buf.backlog)-1])
		}
		buf.backlog[len(buf.backlog)-1] = seg
	} else if latest.Equals(seg) {
		// De-duplicate adjacent identical segments.
		if buf.debug {
			buf.logger.Info("Absorb", "", nil, "removing duplicated segment: %+v", seg)
		}
		// Nothing to do.
	} else {
		if seg.SeqNum > 0 && seg.Flags == 0 && seg.SeqNum == latest.SeqNum+uint32(len(latest.Data)) && len(seg.Data)+len(latest.Data) <= buf.maxSegLen {
			if buf.debug {
				buf.logger.Info("Absorb", "", nil, "merging adjacent data segments: %v", seg)
			}
			buf.backlog[len(buf.backlog)-1] = Segment{
				ID: seg.ID,
				// Sequence number comes from the previous segment.
				SeqNum: latest.SeqNum,
				// Acknowledge number comes from the new segment.
				AckNum: seg.AckNum,
				Data:   append(latest.Data, seg.Data...),
			}
		} else if seg.SeqNum < latest.SeqNum {
			if buf.debug {
				buf.logger.Info("Absorb", "", nil, "clearing backlog to make way for retransmission: %v", seg)
			}
			buf.backlog = make([]Segment, 0, 128)
			buf.backlog = append(buf.backlog, seg)
		} else {
			if buf.debug {
				buf.logger.Info("Absorb", "", nil, "queued segment for outbound over DNS: %v", seg)
			}
			buf.backlog = append(buf.backlog, seg)
		}
	}
}

// First returns the first (oldest) segment, without removing it from the
// backlog.
func (buf *SegmentBuffer) First() (seg Segment, exists bool) {
	buf.mutex.Lock()
	defer buf.mutex.Unlock()
	if len(buf.backlog) == 0 {
		return Segment{}, false
	}
	first := buf.backlog[0]
	return first, true
}

// Pop returns the first segment and removes it from the backlog.
func (buf *SegmentBuffer) Pop() (seg Segment, exists bool) {
	buf.mutex.Lock()
	defer buf.mutex.Unlock()
	if len(buf.backlog) == 0 {
		return Segment{}, false
	}
	first := buf.backlog[0]
	buf.backlog = buf.backlog[1:]
	return first, true
}

// Latest returns the latest segment without removing it from the backlog.
func (buf *SegmentBuffer) Latest() (seg Segment, exists bool) {
	buf.mutex.Lock()
	defer buf.mutex.Unlock()
	if len(buf.backlog) == 0 {
		return Segment{}, false
	}
	return buf.backlog[len(buf.backlog)-1], true
}
