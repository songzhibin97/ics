package ics

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"

	duration "github.com/channelmeter/iso8601duration"
)

type Parser struct {
	parsedCalendars []*Calendar
	parsedEvents    []*Event
	errorsOccured   []error
}

func (p *Parser) GetCalendars() []*Calendar {
	return p.parsedCalendars
}

func (p *Parser) GetEvents() []*Event {
	return p.parsedEvents
}

func (p *Parser) GetErrors() []error {
	return p.errorsOccured
}

func NewParserByUrl(url string) (*Parser, error) {
	resp, err := resty.New().R().Get(url)
	if err != nil {
		return nil, err
	}
	return NewParserByString(resp.String())
}

func NewParserByFile(file string) (*Parser, error) {
	f, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return NewParserByString(string(f))
}

func NewParserByString(str string) (*Parser, error) {
	parser := &Parser{}
	parser.parseICalContent(str, "")
	return parser, nil
}

// parses the iCal formated string to a calendar object
func (p *Parser) parseICalContent(iCalContent, url string) {
	ical := NewCalendar()
	p.parsedCalendars = append(p.parsedCalendars, ical)

	// split the data into calendar info and events data
	eventsData, calInfo := explodeICal(iCalContent)

	// fill the calendar fields
	ical.SetName(p.parseICalName(calInfo))
	ical.SetDesc(p.parseICalDesc(calInfo))
	ical.SetVersion(p.parseICalVersion(calInfo))
	ical.SetTimezone(p.parseICalTimezone(calInfo))
	ical.SetUrl(url)

	// parse the events and add them to ical
	p.parseEvents(ical, eventsData)

}

// explodes the ICal content to array of events and calendar info
func explodeICal(iCalContent string) ([]string, string) {
	reEvents, _ := regexp.Compile(`(BEGIN:VEVENT(.*\n)*?END:VEVENT\r?\n)`)
	allEvents := reEvents.FindAllString(iCalContent, len(iCalContent))
	calInfo := reEvents.ReplaceAllString(iCalContent, "")
	return allEvents, calInfo
}

// parses the iCal Name
func (p *Parser) parseICalName(iCalContent string) string {
	re, _ := regexp.Compile(`X-WR-CALNAME:.*?\n`)
	result := re.FindString(iCalContent)
	return trimField(result, "X-WR-CALNAME:")
}

// parses the iCal description
func (p *Parser) parseICalDesc(iCalContent string) string {
	re, _ := regexp.Compile(`X-WR-CALDESC:.*?\n`)
	result := re.FindString(iCalContent)
	return trimField(result, "X-WR-CALDESC:")
}

// parses the iCal version
func (p *Parser) parseICalVersion(iCalContent string) float64 {
	re, _ := regexp.Compile(`VERSION:.*?\n`)
	result := re.FindString(iCalContent)
	// parse the version result to float
	ver, _ := strconv.ParseFloat(trimField(result, "VERSION:"), 64)
	return ver
}

// parses the iCal timezone
func (p *Parser) parseICalTimezone(iCalContent string) time.Location {
	re, _ := regexp.Compile(`X-WR-TIMEZONE:.*?\n`)
	result := re.FindString(iCalContent)

	// parse the timezone result to time.Location
	timezone := trimField(result, "X-WR-TIMEZONE:")
	// create location instance
	loc, err := time.LoadLocation(timezone)

	// if fails with the timezone => go Local
	if err != nil {
		p.errorsOccured = append(p.errorsOccured, err)
		loc, _ = time.LoadLocation("UTC")
	}
	return *loc
}

// parses the events data
func (p *Parser) parseEvents(cal *Calendar, eventsData []string) {
	for _, eventData := range eventsData {
		event := NewEvent()

		start, startTZID := p.parseEventStart(eventData)
		end, endTZID := p.parseEventEnd(eventData)
		duration := p.parseEventDuration(eventData)

		if end.Before(start) {
			end = start.Add(duration)
		}
		// whole day event when both times are 00:00:00
		wholeDay := start.Hour() == 0 && end.Hour() == 0 && start.Minute() == 0 && end.Minute() == 0 && start.Second() == 0 && end.Second() == 0

		event.SetStartTZID(startTZID)
		event.SetEndTZID(endTZID)
		event.SetStatus(p.parseEventStatus(eventData))
		event.SetSummary(p.parseEventSummary(eventData))
		event.SetDescription(p.parseEventDescription(eventData))
		event.SetImportedID(p.parseEventId(eventData))
		event.SetClass(p.parseEventClass(eventData))
		event.SetSequence(p.parseEventSequence(eventData))
		event.SetCreated(p.parseEventCreated(eventData))
		event.SetLastModified(p.parseEventModified(eventData))
		event.SetRRule(p.parseEventRRule(eventData))
		event.SetLocation(p.parseEventLocation(eventData))
		event.SetGeo(p.parseEventGeo(eventData))
		event.SetStart(start)
		event.SetEnd(end)
		event.SetWholeDayEvent(wholeDay)
		event.SetAttendees(p.parseEventAttendees(eventData))
		event.SetOrganizer(p.parseEventOrganizer(eventData))
		event.SetCalendar(cal)
		event.SetID(event.GenerateEventId())

		cal.SetEvent(*event)
		p.parsedEvents = append(p.parsedEvents, event)

		if event.GetRRule() != "" {

			// until field
			reUntil, _ := regexp.Compile(`UNTIL=(\d)*T(\d)*Z(;){0,1}`)
			untilString := trimField(reUntil.FindString(event.GetRRule()), `(UNTIL=|;)`)
			//  set until date
			var until *time.Time
			if untilString == "" {
				until = nil
			} else {
				untilV, _ := time.Parse(IcsFormat, untilString)
				until = &untilV
			}

			// INTERVAL field
			reInterval, _ := regexp.Compile(`INTERVAL=(\d)*(;){0,1}`)
			intervalString := trimField(reInterval.FindString(event.GetRRule()), `(INTERVAL=|;)`)
			interval, _ := strconv.Atoi(intervalString)

			if interval == 0 {
				interval = 1
			}

			// count field
			reCount, _ := regexp.Compile(`COUNT=(\d)*(;){0,1}`)
			countString := trimField(reCount.FindString(event.GetRRule()), `(COUNT=|;)`)
			count, _ := strconv.Atoi(countString)
			if count == 0 {
				count = MaxRepeats
			}

			// freq field
			reFr, _ := regexp.Compile(`FREQ=[^;]*(;){0,1}`)
			freq := trimField(reFr.FindString(event.GetRRule()), `(FREQ=|;)`)

			// by month field
			reBM, _ := regexp.Compile(`BYMONTH=[^;]*(;){0,1}`)
			bymonth := trimField(reBM.FindString(event.GetRRule()), `(BYMONTH=|;)`)

			// by day field
			reBD, _ := regexp.Compile(`BYDAY=[^;]*(;){0,1}`)
			byday := trimField(reBD.FindString(event.GetRRule()), `(BYDAY=|;)`)

			// fmt.Printf("%#v \n", reBD.FindString(event.GetRRule()))
			// fmt.Println("untilString", reUntil.FindString(event.GetRRule()))

			//  set the freq modification of the dates
			var years, days, months int
			switch freq {
			case "DAILY":
				days = interval
				months = 0
				years = 0
				break
			case "WEEKLY":
				days = 7
				months = 0
				years = 0
				break
			case "MONTHLY":
				days = 0
				months = interval
				years = 0
				break
			case "YEARLY":
				days = 0
				months = 0
				years = interval
				break
			}

			// number of current repeats
			current := 0
			// the current date in the main loop
			freqDateStart := start
			freqDateEnd := end

			// loops by freq
			for {
				weekDaysStart := freqDateStart
				weekDaysEnd := freqDateEnd

				// check repeating by month
				if bymonth == "" || strings.Contains(bymonth, weekDaysStart.Format("1")) {

					if byday != "" {
						// loops the weekdays
						for i := 0; i < 7; i++ {
							day := parseDayNameToIcsName(weekDaysStart.Format("Mon"))
							if strings.Contains(byday, day) && weekDaysStart != start {
								current++
								count--
								newE := *event
								newE.SetStart(weekDaysStart)
								newE.SetEnd(weekDaysEnd)
								newE.SetID(newE.GenerateEventId())
								newE.SetSequence(current)
								if until == nil || (until != nil && until.Format(YmdHis) >= weekDaysStart.Format(YmdHis)) {
									cal.SetEvent(newE)
								}

							}
							weekDaysStart = weekDaysStart.AddDate(0, 0, 1)
							weekDaysEnd = weekDaysEnd.AddDate(0, 0, 1)
						}
					} else {
						//  we dont have loop by day so we put it on the same day
						if weekDaysStart != start {
							current++
							count--
							newE := *event
							newE.SetStart(weekDaysStart)
							newE.SetEnd(weekDaysEnd)
							newE.SetID(newE.GenerateEventId())
							newE.SetSequence(current)
							if until == nil || (until != nil && until.Format(YmdHis) >= weekDaysStart.Format(YmdHis)) {
								cal.SetEvent(newE)
							}
						}
					}
				}

				freqDateStart = freqDateStart.AddDate(years, months, days)
				freqDateEnd = freqDateEnd.AddDate(years, months, days)
				if current > MaxRepeats || count == 0 {
					break
				}

				if until != nil && until.Format(YmdHis) <= freqDateStart.Format(YmdHis) {
					break
				}
			}
		}
	}
}

// parses the event summary
func (p *Parser) parseEventSummary(eventData string) string {
	re, _ := regexp.Compile(`SUMMARY(?:;LANGUAGE=[a-zA-Z\-]+)?.*?\n`)
	result := re.FindString(eventData)
	return trimField(result, `SUMMARY(?:;LANGUAGE=[a-zA-Z\-]+)?:`)
}

// parses the event status
func (p *Parser) parseEventStatus(eventData string) string {
	re, _ := regexp.Compile(`STATUS:.*?\n`)
	result := re.FindString(eventData)
	return trimField(result, "STATUS:")
}

// parses the event description
func (p *Parser) parseEventDescription(eventData string) string {
	re, _ := regexp.Compile(`DESCRIPTION:.*?\n(?:\s+.*?\n)*`)
	result := re.FindString(eventData)
	return trimField(strings.Replace(result, "\r\n ", "", -1), "DESCRIPTION:")
}

// parses the event id provided form google
func (p *Parser) parseEventId(eventData string) string {
	re, _ := regexp.Compile(`UID:.*?\n`)
	result := re.FindString(eventData)
	return trimField(result, "UID:")
}

// parses the event class
func (p *Parser) parseEventClass(eventData string) string {
	re, _ := regexp.Compile(`CLASS:.*?\n`)
	result := re.FindString(eventData)
	return trimField(result, "CLASS:")
}

// parses the event sequence
func (p *Parser) parseEventSequence(eventData string) int {
	re, _ := regexp.Compile(`SEQUENCE:.*?\n`)
	result := re.FindString(eventData)
	sq, _ := strconv.Atoi(trimField(result, "SEQUENCE:"))
	return sq
}

// parses the event created time
func (p *Parser) parseEventCreated(eventData string) time.Time {
	re, _ := regexp.Compile(`CREATED:.*?\n`)
	result := re.FindString(eventData)
	created := trimField(result, "CREATED:")
	t, _ := time.Parse(IcsFormat, created)
	return t
}

// parses the event modified time
func (p *Parser) parseEventModified(eventData string) time.Time {
	re, _ := regexp.Compile(`LAST-MODIFIED:.*?\n`)
	result := re.FindString(eventData)
	modified := trimField(result, "LAST-MODIFIED:")
	t, _ := time.Parse(IcsFormat, modified)
	return t
}

// parses the event start time
func (p *Parser) parseTimeField(fieldName string, eventData string) (time.Time, string) {
	reWholeDay, _ := regexp.Compile(fmt.Sprintf(`%s;VALUE=DATE:.*?\n`, fieldName))
	re, _ := regexp.Compile(fmt.Sprintf(`%s(;TZID=(.*?))?(;VALUE=DATE-TIME)?:(.*?)\n`, fieldName))
	resultWholeDay := reWholeDay.FindString(eventData)
	var t time.Time
	var tzID string

	if resultWholeDay != "" {
		// whole day event
		modified := trimField(resultWholeDay, fmt.Sprintf("%s;VALUE=DATE:", fieldName))
		t, _ = time.Parse(IcsFormatWholeDay, modified)
	} else {
		// event that has start hour and minute
		result := re.FindStringSubmatch(eventData)
		if result == nil || len(result) < 4 {
			return t, tzID
		}
		tzID = result[2]
		dt := result[4]
		if !strings.Contains(dt, "Z") {
			dt = fmt.Sprintf("%sZ", dt)
		}
		t, _ = time.Parse(IcsFormat, dt)
	}

	return t, tzID
}

// parses the event start time
func (p *Parser) parseEventStart(eventData string) (time.Time, string) {
	return p.parseTimeField("DTSTART", eventData)
}

// parses the event end time
func (p *Parser) parseEventEnd(eventData string) (time.Time, string) {
	return p.parseTimeField("DTEND", eventData)
}

func (p *Parser) parseEventDuration(eventData string) time.Duration {
	reDuration, _ := regexp.Compile(`DURATION:.*?\n`)
	result := reDuration.FindString(eventData)
	trimmed := trimField(result, "DURATION:")
	parsedDuration, err := duration.FromString(trimmed)
	var output time.Duration

	if err == nil {
		output = parsedDuration.ToDuration()
	}

	return output
}

// parses the event RRULE (the repeater)
func (p *Parser) parseEventRRule(eventData string) string {
	re, _ := regexp.Compile(`RRULE:.*?\n`)
	result := re.FindString(eventData)
	return trimField(result, "RRULE:")
}

// parses the event LOCATION
func (p *Parser) parseEventLocation(eventData string) string {
	re, _ := regexp.Compile(`LOCATION:.*?\n`)
	result := re.FindString(eventData)
	return trimField(result, "LOCATION:")
}

// parses the event GEO
func (p *Parser) parseEventGeo(eventData string) *Geo {
	re, _ := regexp.Compile(`GEO:.*?\n`)
	result := re.FindString(eventData)

	value := trimField(result, "GEO:")
	values := strings.Split(value, ";")
	if len(values) < 2 {
		return nil
	}

	return NewGeo(values[0], values[1])
}

// parses the event attendees
func (p *Parser) parseEventAttendees(eventData string) []*Attendee {
	attendeesObj := []*Attendee{}
	re, _ := regexp.Compile(`ATTENDEE(:|;)(.*?\r?\n)(\s.*?\r?\n)*`)
	attendees := re.FindAllString(eventData, len(eventData))

	for _, attendeeData := range attendees {
		if attendeeData == "" {
			continue
		}
		attendee := p.parseAttendee(strings.Replace(strings.Replace(attendeeData, "\r", "", 1), "\n ", "", 1))
		//  check for any fields set
		if attendee.GetEmail() != "" || attendee.GetName() != "" || attendee.GetRole() != "" || attendee.GetStatus() != "" || attendee.GetType() != "" {
			attendeesObj = append(attendeesObj, attendee)
		}
	}
	return attendeesObj
}

// parses the event organizer
func (p *Parser) parseEventOrganizer(eventData string) *Attendee {

	re, _ := regexp.Compile(`ORGANIZER(:|;)(.*?\r?\n)(\s.*?\r?\n)*`)
	organizerData := re.FindString(eventData)
	if organizerData == "" {
		return nil
	}
	organizerDataFormated := strings.Replace(strings.Replace(organizerData, "\r", "", 1), "\n ", "", 1)

	a := NewAttendee()
	a.SetEmail(p.parseAttendeeMail(organizerDataFormated))
	a.SetName(p.parseOrganizerName(organizerDataFormated))

	return a
}

// parse attendee properties
func (p *Parser) parseAttendee(attendeeData string) *Attendee {

	a := NewAttendee()
	a.SetEmail(p.parseAttendeeMail(attendeeData))
	a.SetName(p.parseAttendeeName(attendeeData))
	a.SetRole(p.parseAttendeeRole(attendeeData))
	a.SetStatus(p.parseAttendeeStatus(attendeeData))
	a.SetType(p.parseAttendeeType(attendeeData))
	return a
}

// parses the attendee email
func (p *Parser) parseAttendeeMail(attendeeData string) string {
	re, _ := regexp.Compile(`mailto:.*?\n`)
	result := re.FindString(attendeeData)
	return trimField(result, "mailto:")
}

// parses the attendee status
func (p *Parser) parseAttendeeStatus(attendeeData string) string {
	re, _ := regexp.Compile(`PARTSTAT=.*?;`)
	result := re.FindString(attendeeData)
	if result == "" {
		return ""
	}
	return trimField(result, `(PARTSTAT=|;)`)
}

// parses the attendee role
func (p *Parser) parseAttendeeRole(attendeeData string) string {
	re, _ := regexp.Compile(`ROLE=.*?;`)
	result := re.FindString(attendeeData)

	if result == "" {
		return ""
	}
	return trimField(result, `(ROLE=|;)`)
}

// parses the attendee Name
func (p *Parser) parseAttendeeName(attendeeData string) string {
	re, _ := regexp.Compile(`CN=.*?;`)
	result := re.FindString(attendeeData)
	if result == "" {
		return ""
	}
	return trimField(result, `(CN=|;)`)
}

// parses the organizer Name
func (p *Parser) parseOrganizerName(orgData string) string {
	re, _ := regexp.Compile(`CN=.*?:`)
	result := re.FindString(orgData)
	if result == "" {
		return ""
	}
	return trimField(result, `(CN=|:)`)
}

// parses the attendee type
func (p *Parser) parseAttendeeType(attendeeData string) string {
	re, _ := regexp.Compile(`CUTYPE=.*?;`)
	result := re.FindString(attendeeData)
	if result == "" {
		return ""
	}
	return trimField(result, `(CUTYPE=|;)`)
}
