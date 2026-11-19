package ses_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	"github.com/aws/smithy-go"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/notification/internal/adapter/ses"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/notification/internal/app"
)

// fakeSES stands in for SES: it keeps the input of each call, and answers
// with err. With block, it answers only when the context of the call ends.
type fakeSES struct {
	mu     sync.Mutex
	inputs []*sesv2.SendEmailInput
	err    error
	block  bool
}

func (f *fakeSES) SendEmail(ctx context.Context, in *sesv2.SendEmailInput, _ ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error) {
	f.mu.Lock()
	f.inputs = append(f.inputs, in)
	err, block := f.err, f.block
	f.mu.Unlock()
	if block {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, err
	}
	return &sesv2.SendEmailOutput{MessageId: aws.String("0100019a-test")}, nil
}

func (f *fakeSES) calls() []*sesv2.SendEmailInput {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*sesv2.SendEmailInput(nil), f.inputs...)
}

func newMailer(t *testing.T, client ses.Client, configure func(*ses.Config)) *ses.Mailer {
	t.Helper()
	c := ses.Config{Client: client, From: "Meeting rooms <rooms@example.com>"}
	if configure != nil {
		configure(&c)
	}
	m, err := ses.New(c)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// message is the message of the tests of the adapter smtp.
func message() app.Message {
	return app.Message{
		To:      "organizer@example.com",
		Subject: "Reservation confirmed: Fri, 20 Nov 2026 01:00 UTC",
		Body: "Your reservation of a meeting room is confirmed.\n\n" +
			"Reservation: 0193a6f0-7b5c-7e2a-9c3d-1f2e3d4c5b6a\n" +
			".a line that starts with a dot\n" +
			"a=b, and a line long enough to be folded by quoted-printable, which allows 76 characters on a line\n",
	}
}

// request is what SendEmail receives, in the words of the tests.
func request(from, to, subject, body string) *sesv2.SendEmailInput {
	return &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(from),
		Destination:      &types.Destination{ToAddresses: []string{to}},
		Content: &types.EmailContent{
			Simple: &types.Message{
				Subject: &types.Content{Data: aws.String(subject), Charset: aws.String("UTF-8")},
				Body:    &types.Body{Text: &types.Content{Data: aws.String(body), Charset: aws.String("UTF-8")}},
			},
		},
	}
}

func show(t *testing.T, v any) string {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The request names the sender, the one recipient, the subject, and the
// body, as Simple content in UTF-8, and nothing else: no copy, no other
// content, no configuration set, no tags.
func TestSendHandsSESTheMessage(t *testing.T) {
	t.Parallel()
	client := &fakeSES{}
	msg := message()
	if err := newMailer(t, client, nil).Send(t.Context(), msg); err != nil {
		t.Fatal(err)
	}
	calls := client.calls()
	if len(calls) != 1 {
		t.Fatalf("SES was called %d times, want once", len(calls))
	}
	want := request(`"Meeting rooms" <rooms@example.com>`, "organizer@example.com", msg.Subject, msg.Body)
	if !reflect.DeepEqual(calls[0], want) {
		t.Errorf("SendEmail received\n%s\nwant\n%s", show(t, calls[0]), show(t, want))
	}
}

// SES takes text in UTF-8 as it is, and a name in an address only as an
// encoded word of RFC 2047.
func TestTextAndNamesThatAreNotASCII(t *testing.T) {
	t.Parallel()
	client := &fakeSES{}
	mailer := newMailer(t, client, func(c *ses.Config) { c.From = "会議室 <rooms@example.com>" })
	msg := app.Message{To: "Ann <ann@example.com>", Subject: "会議室の予約が確定しました", Body: "予約が確定しました。\n"}
	if err := mailer.Send(t.Context(), msg); err != nil {
		t.Fatal(err)
	}
	calls := client.calls()
	if len(calls) != 1 {
		t.Fatalf("SES was called %d times, want once", len(calls))
	}
	got := calls[0]
	from := aws.ToString(got.FromEmailAddress)
	name, address, found := strings.Cut(from, " ")
	if !found || address != "<rooms@example.com>" || !strings.HasPrefix(name, "=?utf-8?") {
		t.Fatalf("the sender is %q, want an encoded name and <rooms@example.com>", from)
	}
	var decoder mime.WordDecoder
	if decoded, err := decoder.DecodeHeader(name); err != nil || decoded != "会議室" {
		t.Errorf("the name of the sender decodes to %q (%v), want 会議室", decoded, err)
	}
	if to := got.Destination.ToAddresses; len(to) != 1 || to[0] != `"Ann" <ann@example.com>` {
		t.Errorf("the recipients are %q, want the one with its name", to)
	}
	simple := got.Content.Simple
	if aws.ToString(simple.Subject.Data) != msg.Subject || aws.ToString(simple.Body.Text.Data) != msg.Body {
		t.Errorf("the subject and the body are %q and %q, want them as they are", aws.ToString(simple.Subject.Data), aws.ToString(simple.Body.Text.Data))
	}
}

// A line break in an address or in the subject would let the text after it
// pass as another header, such as a Bcc. Send refuses it, and an address that
// SES does not take, and calls SES for none of them.
func TestSendRefusesWhatSESMustNotReceive(t *testing.T) {
	t.Parallel()
	client := &fakeSES{}
	mailer := newMailer(t, client, nil)
	tests := map[string]func(*app.Message){
		"CR LF in the subject":     func(m *app.Message) { m.Subject = "Hello\r\nBcc: victim@example.com" },
		"LF in the subject":        func(m *app.Message) { m.Subject = "Hello\nBcc: victim@example.com" },
		"CR in the subject":        func(m *app.Message) { m.Subject = "Hello\rBcc: victim@example.com" },
		"CR LF in the recipient":   func(m *app.Message) { m.To = "organizer@example.com\r\nBcc: victim@example.com" },
		"LF in the recipient":      func(m *app.Message) { m.To = "organizer@example.com\nBcc: victim@example.com" },
		"two recipients":           func(m *app.Message) { m.To = "organizer@example.com, victim@example.com" },
		"no recipient":             func(m *app.Message) { m.To = "" },
		"a recipient not in ASCII": func(m *app.Message) { m.To = "organizer@例え.jp" },
	}
	for name, change := range tests {
		msg := message()
		change(&msg)
		if err := mailer.Send(t.Context(), msg); err == nil {
			t.Errorf("%s: Send = nil, want an error", name)
		}
	}
	if n := len(client.calls()); n != 0 {
		t.Errorf("SES was called %d times, want never", n)
	}
}

func TestNewRefusesAnIncompleteConfig(t *testing.T) {
	t.Parallel()
	client := &fakeSES{}
	tests := map[string]ses.Config{
		"no client":             {From: "rooms@example.com"},
		"no sender":             {Client: client},
		"a sender no address":   {Client: client, From: "rooms"},
		"a line break in From":  {Client: client, From: "rooms@example.com\r\nBcc: victim@example.com"},
		"two senders":           {Client: client, From: "a@example.com, b@example.com"},
		"a sender with a break": {Client: client, From: "Rooms\n <rooms@example.com>"},
		"a sender not in ASCII": {Client: client, From: "rooms@例え.jp"},
	}
	for name, c := range tests {
		if _, err := ses.New(c); err == nil {
			t.Errorf("%s: New = nil error, want one", name)
		}
	}
}

// Every failure of SES fails Send, so that the event comes again, and keeps
// the answer of SES in the error: a refusal that will come again as well as
// a throttling or an error of the service.
func TestSendFailsWithTheAnswerOfSES(t *testing.T) {
	t.Parallel()
	tests := []struct {
		answer error
		code   string
	}{
		{&types.MessageRejected{Message: aws.String("Email address is not verified.")}, "MessageRejected"},
		{&types.MailFromDomainNotVerifiedException{}, "MailFromDomainNotVerifiedException"},
		{&types.SendingPausedException{}, "SendingPausedException"},
		{&types.AccountSuspendedException{}, "AccountSuspendedException"},
		{&types.TooManyRequestsException{}, "TooManyRequestsException"},
		{&types.LimitExceededException{}, "LimitExceededException"},
		{&smithy.GenericAPIError{Code: "InternalFailure", Fault: smithy.FaultServer}, "InternalFailure"},
	}
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			t.Parallel()
			err := newMailer(t, &fakeSES{err: tt.answer}, nil).Send(t.Context(), message())
			var answer smithy.APIError
			if !errors.As(err, &answer) || answer.ErrorCode() != tt.code {
				t.Fatalf("Send = %v, want the answer %s of SES in the error", err, tt.code)
			}
			if !strings.Contains(err.Error(), tt.code) {
				t.Errorf("Send = %q, want an error that names %s", err, tt.code)
			}
		})
	}
}

// A call that SES does not answer holds Send no longer than the deadline of
// its context, or the timeout of the mailer.
func TestSendGivesUpWhenSESDoesNotAnswer(t *testing.T) {
	t.Parallel()
	const wait = 200 * time.Millisecond
	tests := []struct {
		name      string
		configure func(*ses.Config)
		// deadline and cancel end the context of Send after that long, when
		// they are not zero.
		deadline, cancel time.Duration
		want             error
	}{
		{name: "the deadline of the context", deadline: wait, want: context.DeadlineExceeded},
		{name: "the cancellation of the context", cancel: wait, want: context.Canceled},
		{
			name:      "the timeout of the mailer",
			configure: func(c *ses.Config) { c.Timeout = wait },
			want:      context.DeadlineExceeded,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mailer := newMailer(t, &fakeSES{block: true}, tt.configure)
			ctx := t.Context()
			if tt.deadline > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, tt.deadline)
				defer cancel()
			}
			if tt.cancel > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				defer cancel()
				time.AfterFunc(tt.cancel, cancel)
			}
			start := time.Now()
			err := mailer.Send(ctx, message())
			if elapsed := time.Since(start); elapsed > 5*time.Second {
				t.Errorf("Send took %s, want about 200ms", elapsed)
			}
			if !errors.Is(err, tt.want) {
				t.Errorf("Send = %v, want %v", err, tt.want)
			}
		})
	}
}

// answer is one answer of the fake endpoint of SES: an HTTP status, and for
// an error, its code.
type answer struct {
	status int
	code   string
}

// endpoint is an HTTP server that answers each call of SendEmail with the
// next answer, and keeps the body of each request.
type endpoint struct {
	mu       sync.Mutex
	answers  []answer
	requests []string
}

func (e *endpoint) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	e.mu.Lock()
	n := len(e.requests)
	e.requests = append(e.requests, r.Method+" "+r.URL.Path+" "+string(body))
	next := answer{status: http.StatusOK}
	if n < len(e.answers) {
		next = e.answers[n]
	}
	e.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if next.status == http.StatusOK {
		_, _ = io.WriteString(w, `{"MessageId":"0100019a-test"}`)
		return
	}
	if next.code != "" {
		w.Header().Set("X-Amzn-ErrorType", next.code)
	}
	w.WriteHeader(next.status)
	_, _ = io.WriteString(w, `{"message":"an answer of the test"}`)
}

func (e *endpoint) received() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.requests...)
}

// sdkClient returns the client of the SDK, as sesv2.NewFromConfig makes it
// in the worker, for the endpoint. Its retryer is the default one, without
// the pause between attempts.
func sdkClient(t *testing.T, e *endpoint) *sesv2.Client {
	t.Helper()
	server := httptest.NewServer(e)
	t.Cleanup(server.Close)
	return sesv2.NewFromConfig(aws.Config{
		Region:       "us-east-1",
		BaseEndpoint: aws.String(server.URL),
		Credentials:  credentials.NewStaticCredentialsProvider("test", "test", ""),
		Retryer: func() aws.Retryer {
			return retry.NewStandard(func(o *retry.StandardOptions) {
				o.Backoff = retry.BackoffDelayerFunc(func(int, error) (time.Duration, error) { return 0, nil })
			})
		},
	})
}

// The request on the wire carries the message, as the API of SES v2 names
// its fields.
func TestTheSDKSendsTheMessageToSES(t *testing.T) {
	t.Parallel()
	e := &endpoint{}
	msg := message()
	if err := newMailer(t, sdkClient(t, e), nil).Send(t.Context(), msg); err != nil {
		t.Fatal(err)
	}
	requests := e.received()
	if len(requests) != 1 {
		t.Fatalf("SES received %d requests, want one", len(requests))
	}
	line, body, _ := strings.Cut(requests[0], " {")
	if line != "POST /v2/email/outbound-emails" {
		t.Errorf("the request is %q, want POST /v2/email/outbound-emails", line)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte("{"+body), &got); err != nil {
		t.Fatalf("the body of the request is no JSON: %v\n%s", err, requests[0])
	}
	content := func(data string) map[string]any { return map[string]any{"Data": data, "Charset": "UTF-8"} }
	want := map[string]any{
		"FromEmailAddress": `"Meeting rooms" <rooms@example.com>`,
		"Destination":      map[string]any{"ToAddresses": []any{"organizer@example.com"}},
		"Content": map[string]any{
			"Simple": map[string]any{
				"Subject": content(msg.Subject),
				"Body":    map[string]any{"Text": content(msg.Body)},
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("the body of the request is\n%s\nwant\n%s", show(t, got), show(t, want))
	}
}

// The SDK tries a throttling and an error of the service again, within one
// Send, and a refusal not: SES would refuse it again at once.
func TestTheSDKTriesAgainWhatMayPassLater(t *testing.T) {
	t.Parallel()
	throttling := answer{http.StatusTooManyRequests, "TooManyRequestsException"}
	tests := []struct {
		name     string
		answers  []answer
		requests int
		// fails is the code of the answer in the error of Send, or "" when
		// Send succeeds.
		fails string
	}{
		{"a throttling, then success", []answer{throttling}, 2, ""},
		{"a quota, then success", []answer{{http.StatusBadRequest, "LimitExceededException"}}, 2, ""},
		{"an error of the service, then success", []answer{{http.StatusServiceUnavailable, ""}}, 2, ""},
		{"throttling at every attempt", []answer{throttling, throttling, throttling}, 3, "TooManyRequestsException"},
		{"a refusal", []answer{{http.StatusBadRequest, "MessageRejected"}}, 1, "MessageRejected"},
		{"an account whose sending is paused", []answer{{http.StatusBadRequest, "SendingPausedException"}}, 1, "SendingPausedException"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := &endpoint{answers: tt.answers}
			err := newMailer(t, sdkClient(t, e), nil).Send(t.Context(), message())
			if n := len(e.received()); n != tt.requests {
				t.Errorf("SES received %d requests, want %d", n, tt.requests)
			}
			if tt.fails == "" {
				if err != nil {
					t.Errorf("Send = %v, want nil", err)
				}
				return
			}
			var answer smithy.APIError
			if !errors.As(err, &answer) || answer.ErrorCode() != tt.fails {
				t.Errorf("Send = %v, want the answer %s of SES in the error", err, tt.fails)
			}
		})
	}
}
