package internal

import (
	"log"
	"time"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/oklog/ulid/v2"
)

type irateLimitingWindow interface {
	allowRequest(rInfo slidingWindowRequestInfo) (ok bool)
	//enrichRequestAfterHandling(reqInfo *IncomingRequestInfo)
}

type rateLimitingWindowParameters struct {
	duration     time.Duration
	requestCount int
}

// sharedRateLimitingWindow is shared between several sockets.
type sharedRateLimitingWindow struct {
	*rateLimitingSlidingWindow
}

func newSharedRateLimitingWindow(params rateLimitingWindowParameters) *sharedRateLimitingWindow {
	if params.requestCount <= 0 {
		log.Panicln("cannot create sliding window with request count less or equal to zero")
	}

	window := &sharedRateLimitingWindow{
		newRateLimitingSlidingWindow(params),
	}

	return window
}

func (window *sharedRateLimitingWindow) allowRequest(req slidingWindowRequestInfo) (ok bool) {
	prevReqCount := 0
	sockets := make([]string, 0)

	for _, windowReq := range window.rateLimitingSlidingWindow.requests {

		// ignore "old" requests
		if req.creationTime.Sub(windowReq.creationTime) >= window.duration {
			continue
		}

		if !utils.SliceContains(sockets, windowReq.remoteAddrAndPort) {
			sockets = append(sockets, windowReq.remoteAddrAndPort)
		}

		if windowReq.remoteAddrAndPort == req.remoteAddrAndPort {
			prevReqCount += 1
		}
	}

	reqCountF := float32(prevReqCount + 1)
	totalReqCountF := float32(len(window.requests))
	maxSocketReqCount := totalReqCountF / float32(len(sockets))

	//socket has exceeded its share
	ok = (len(sockets) == 1 && reqCountF < totalReqCountF/2) || (len(sockets) != 1 && reqCountF <= maxSocketReqCount)

	if !window.rateLimitingSlidingWindow.allowRequest(req) {
		ok = false
	}

	return
}

type rateLimitingSlidingWindow struct {
	duration    time.Duration
	requests    []slidingWindowRequestInfo
	burstWindow irateLimitingWindow
}

type slidingWindowRequestInfo struct {
	ulid              ulid.ULID //should not be used to retrieve time of request
	method            string
	creationTime      time.Time
	remoteAddrAndPort string
	remoteIpAddr      string
	sentBytes         int
}

func (info slidingWindowRequestInfo) IsMutation() bool {
	return info.method == "POST" || info.method == "PATCH" || info.method == "DELETE"
}

func newRateLimitingSlidingWindow(params rateLimitingWindowParameters) *rateLimitingSlidingWindow {

	if params.requestCount <= 0 {
		log.Panicln("cannot create sliding window with request count less or equal to zero")
	}

	window := &rateLimitingSlidingWindow{
		duration:    params.duration,
		requests:    make([]slidingWindowRequestInfo, params.requestCount),
		burstWindow: nil,
	}

	for i := range window.requests {
		window.requests[i].ulid = ulid.ULID{}
	}

	return window
}

func (window *rateLimitingSlidingWindow) allowRequest(rInfo slidingWindowRequestInfo) (ok bool) {
	candidateSlotIndexes := make([]int, 0)

	//if we find an empty slot for the request we accept it immediately
	//otherwise we search for slots that contain "old" requests.
	for i, req := range window.requests {

		if req.ulid == (ulid.ULID{}) { //empty slot
			window.requests[i] = rInfo
			return true
		}

		if rInfo.creationTime.Sub(req.creationTime) > window.duration {
			candidateSlotIndexes = append(candidateSlotIndexes, i)
		}
	}

	switch len(candidateSlotIndexes) {
	case 0:
		oldestRequestTime := window.requests[0].creationTime
		oldestRequestSlotIndex := 0
		for i, req := range window.requests {
			if req.creationTime.Before(oldestRequestTime) {
				oldestRequestTime = req.creationTime
				oldestRequestSlotIndex = i
			}
		}

		window.requests[oldestRequestSlotIndex] = rInfo
		return window.burstWindow != nil && window.burstWindow.allowRequest(rInfo)
	case 1:
		window.requests[candidateSlotIndexes[0]] = rInfo
		return true
	default:
		oldestRequestTime := window.requests[candidateSlotIndexes[0]].creationTime
		oldestRequestSlotIndex := candidateSlotIndexes[0]
		for _, slotIndex := range candidateSlotIndexes[1:] {
			requestTime := window.requests[slotIndex].creationTime

			if requestTime.Before(oldestRequestTime) {
				oldestRequestTime = requestTime
				oldestRequestSlotIndex = slotIndex
			}
		}

		window.requests[oldestRequestSlotIndex] = rInfo
		return true
	}
}
