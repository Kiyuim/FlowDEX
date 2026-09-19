package constants

type KlineInterval string

var (
	KlineMin1   KlineInterval = "1m"
	KlineMin5   KlineInterval = "5m"
	KlineMin15  KlineInterval = "15m"
	KlineHour1  KlineInterval = "1h"
	KlineHour4  KlineInterval = "4h"
	KlineHour12 KlineInterval = "12h"
	KlineDay1   KlineInterval = "1d"
)

func (ki KlineInterval) String() string {
	return string(ki)
}

var KlineIntervals = []KlineInterval{
	KlineMin1,
	KlineMin5,
	KlineMin15,
	KlineHour1,
	KlineHour4,
	KlineHour12,
	KlineDay1,
}

func MinuteToInterval(interval int) KlineInterval {
	switch interval {
	case 1:
		return KlineMin1
	case 5:
		return KlineMin5
	case 15:
		return KlineMin15
	case 60:
		return KlineHour1
	case 240:
		return KlineHour4
	case 720:
		return KlineHour12
	case 1440:
		return KlineDay1
	default:
		return ""
	}
}

type KlineIntervalSecond int64

const (
	IntervalMin1   KlineIntervalSecond = 1 * 60
	IntervalMin5   KlineIntervalSecond = 5 * 60
	IntervalMin15  KlineIntervalSecond = 15 * 60
	IntervalMin30  KlineIntervalSecond = 30 * 60
	IntervalHour1  KlineIntervalSecond = 1 * 60 * 60
	IntervalHour4  KlineIntervalSecond = 4 * 60 * 60
	IntervalHour6  KlineIntervalSecond = 6 * 60 * 60
	IntervalHour12 KlineIntervalSecond = 12 * 60 * 60
	IntervalHour24 KlineIntervalSecond = 24 * 60 * 60
)

const (
	KlineIntervalMin1   int = 1
	KlineIntervalMin5   int = 5
	KlineIntervalMin15  int = 15
	KlineIntervalHour1  int = 60
	KlineIntervalHour4  int = 4 * 60
	KlineIntervalHour6  int = 6 * 60
	KlineIntervalHour12 int = 12 * 60
	KlineIntervalHour24 int = 24 * 60
)

var KlineIntervalSecondsMap = map[string]KlineIntervalSecond{
	"1m":  IntervalMin1,
	"5m":  IntervalMin5,
	"15m": IntervalMin15,
	"1h":  IntervalHour1,
	"4h":  IntervalHour4,
	"12h": IntervalHour12,
	"1d":  IntervalHour24,
}

var KlineIntervalMinutes = []int{
	KlineIntervalMin1,
	KlineIntervalMin5,
	KlineIntervalMin15,
	KlineIntervalHour1,
	KlineIntervalHour4,
	KlineIntervalHour12,
	KlineIntervalHour24,
}
