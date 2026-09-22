package defined

// DateFormat is the calendar-date layout used on the wire and in the CSV
// (commissioned_date). Ambiguous day/month layouts (01/02/2020) are
// deliberately not accepted anywhere.
const DateFormat = "2006-01-02"
