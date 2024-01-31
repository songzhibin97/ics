package ics

import (
	"regexp"
	"strings"
)

// unixtimestamp
const uts = "1136239445"

// ics date time format
const IcsFormat = "20060102T150405Z"

// Y-m-d H:i:S time format
const YmdHis = "2006-01-02 15:04:05"

// ics date format ( describes a whole day)
const IcsFormatWholeDay = "20060102"

const MaxRepeats = 10

// removes newlines and cutset from given string
func trimField(field, cutset string) string {
	re, _ := regexp.Compile(cutset)
	cutsetRem := re.ReplaceAllString(field, "")
	return strings.TrimRight(cutsetRem, "\r\n")
}

func stringToByte(str string) []byte {
	return []byte(str)
}

func parseDayNameToIcsName(day string) string {
	switch day {
	case "Mon":
		return "MO"
	case "Tue":
		return "TU"
	case "Wed":
		return "WE"
	case "Thu":
		return "TH"

	case "Fri":
		return "FR"
	case "Sat":
		return "ST"
	case "Sun":
		return "SU"
	default:
		return ""
	}
}
