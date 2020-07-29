package common

import (
	"flag"
	"testing"
	"time"

	"github.com/golib/assert"
)

var stopAtTime = flag.String("stopAt", "6", "stop time")

func TestIsStopTime(t *testing.T) {

	now := time.Date(0, 0, 0, 11, 30, 0, 0, time.Local)
	*stopAtTime = "12:"
	stopTimeSlice, isVaild := IsValidWithStopTime(stopAtTime)
	assert.Equal(t, true, isVaild)
	assert.Equal(t, false, IsStopTime(now, stopTimeSlice))

	*stopAtTime = "12"
	stopTimeSlice, isVaild = IsValidWithStopTime(stopAtTime)
	assert.Equal(t, true, isVaild)
	assert.Equal(t, false, IsStopTime(now, stopTimeSlice))

	*stopAtTime = "12:00"
	stopTimeSlice, isVaild = IsValidWithStopTime(stopAtTime)
	assert.Equal(t, true, isVaild)
	assert.Equal(t, false, IsStopTime(now, stopTimeSlice))

	*stopAtTime = "11"
	stopTimeSlice, isVaild = IsValidWithStopTime(stopAtTime)
	assert.Equal(t, true, isVaild)
	assert.Equal(t, true, IsStopTime(now, stopTimeSlice))

	*stopAtTime = "11:"
	stopTimeSlice, isVaild = IsValidWithStopTime(stopAtTime)
	assert.Equal(t, true, isVaild)
	assert.Equal(t, true, IsStopTime(now, stopTimeSlice))

	*stopAtTime = "11:00"
	stopTimeSlice, isVaild = IsValidWithStopTime(stopAtTime)
	assert.Equal(t, true, isVaild)
	assert.Equal(t, true, IsStopTime(now, stopTimeSlice))

	*stopAtTime = "11:59"
	stopTimeSlice, isVaild = IsValidWithStopTime(stopAtTime)
	assert.Equal(t, true, isVaild)
	assert.Equal(t, false, IsStopTime(now, stopTimeSlice))

	*stopAtTime = "11:20"
	stopTimeSlice, isVaild = IsValidWithStopTime(stopAtTime)
	assert.Equal(t, true, isVaild)
	assert.Equal(t, true, IsStopTime(now, stopTimeSlice))

	*stopAtTime = "1e:3t"
	stopTimeSlice, isVaild = IsValidWithStopTime(stopAtTime)
	assert.Equal(t, false, isVaild)
	assert.Equal(t, true, IsStopTime(now, stopTimeSlice))

	*stopAtTime = "1e"
	stopTimeSlice, isVaild = IsValidWithStopTime(stopAtTime)
	assert.Equal(t, false, isVaild)
	assert.Equal(t, true, IsStopTime(now, stopTimeSlice))

	*stopAtTime = "30"
	stopTimeSlice, isVaild = IsValidWithStopTime(stopAtTime)
	assert.Equal(t, false, isVaild)
	assert.Equal(t, true, IsStopTime(now, stopTimeSlice))

	*stopAtTime = "11:70"
	stopTimeSlice, isVaild = IsValidWithStopTime(stopAtTime)
	assert.Equal(t, false, isVaild)
	assert.Equal(t, true, IsStopTime(now, stopTimeSlice))

	*stopAtTime = "9:30"
	stopTimeSlice, isVaild = IsValidWithStopTime(stopAtTime)
	assert.Equal(t, true, isVaild)
	assert.Equal(t, true, IsStopTime(now, stopTimeSlice))
}
