package common

import (
	"strconv"
	"strings"
	"time"

	"github.com/qiniu/log.v1"
)

func IsValidWithStopTime(stopAtTime *string) (ret []int, isVaild bool) {

	stopTimeSlice := strings.Split(*stopAtTime, ":")
	if len(stopTimeSlice) > 2 {
		return nil, false
	}
	hour, err := strconv.Atoi(stopTimeSlice[0])
	if err != nil || (hour < 0 || hour > 24) {
		return nil, false
	}
	ret = append(ret, hour)
	if len(stopTimeSlice) == 2 && stopTimeSlice[1] != "" {
		min, err := strconv.Atoi(stopTimeSlice[1])
		if err != nil || (min < 0 || min > 60) {
			return nil, false
		}
		ret = append(ret, min)
	}

	return ret, true
}

func IsStopTime(now time.Time, stopTimeSlice []int) bool {

	if stopTimeSlice == nil {
		return true
	}
	if now.Hour() > stopTimeSlice[0] {
		log.Info("stop time")
		return true
	} else if now.Hour() == stopTimeSlice[0] {
		if len(stopTimeSlice) == 2 {
			if now.Minute() >= stopTimeSlice[1] {
				log.Info("stop time")
				return true
			}
			return false
		}
		log.Info("stop time")
		return true
	}
	return false
}
