package message

// ID represents Gmail Message ID.
type ID string

// EmailMessage represents an email message we obtain from Gmail API but
// independent from any external SDK. It only contains the data the domain
// layer cares about.
type EmailMessage struct {
	ID      ID
	Subject string
	Body    string // plain text body extracted & aggregated
}
