package evalsubstrate

import "time"

var timeNow = time.Now

func timeNowUnix() int64 { return timeNow().Unix() }
