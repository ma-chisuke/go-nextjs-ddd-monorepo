//go:build integration

// These tests run the adapter against SES in AWS Emulator (`make
// aws-server`), with the configuration of package awsconfig, as the worker
// calls it. They are also the tests of the emulator itself (TPL-0017): they
// fail when a version of it stops doing what the template relies on, which
// is that SendEmail of the SES API v2 sends from an address that a verified
// identity covers, and refuses any other sender with MessageRejected.
//
// The emulator keeps no mail that a test could read back, so these tests
// show that the request reaches SES and how SES answers it. What the request
// holds is the subject of the unit tests, which read it on the wire.
package ses_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	"github.com/google/uuid"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/notification/internal/adapter/ses"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/awsconfig/awstest"
)

// domain returns a domain that no identity in the emulator covers. The
// emulator keeps one state for every test and every checkout, and a verified
// domain covers its subdomains too, so the domain is new under a top-level
// domain that nobody verifies.
func domain() string {
	return "ses-" + uuid.NewString() + ".test"
}

// verify creates an identity of SES for the domain, waits until the emulator
// has verified it, and deletes it when the test ends. The emulator verifies a
// new identity after about half a second; SES in AWS waits for the records of
// the domain in DNS.
func verify(t *testing.T, client *sesv2.Client, domain string) {
	t.Helper()
	if _, err := client.CreateEmailIdentity(t.Context(), &sesv2.CreateEmailIdentityInput{EmailIdentity: aws.String(domain)}); err != nil {
		t.Fatalf("cannot create an identity in AWS Emulator; run `make aws-server` first: %v", err)
	}
	t.Cleanup(func() {
		// A fresh context: the context of the test is canceled before Cleanup
		// runs. A failure is of no interest: the emulator forgets everything
		// when it stops.
		_, _ = client.DeleteEmailIdentity(context.Background(), &sesv2.DeleteEmailIdentityInput{EmailIdentity: aws.String(domain)})
	})
	deadline := time.Now().Add(30 * time.Second)
	for {
		out, err := client.GetEmailIdentity(t.Context(), &sesv2.GetEmailIdentityInput{EmailIdentity: aws.String(domain)})
		if err != nil {
			t.Fatal(err)
		}
		if out.VerifiedForSendingStatus {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("after 30 seconds, the identity %s is still %s", domain, out.VerificationStatus)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func TestSendThroughAWSEmulator(t *testing.T) {
	client := sesv2.NewFromConfig(awstest.Config(t))
	sender := domain()
	verify(t, client, sender)
	mailer, err := ses.New(ses.Config{Client: client, From: "Meeting rooms <rooms@" + sender + ">"})
	if err != nil {
		t.Fatal(err)
	}
	if err := mailer.Send(t.Context(), message()); err != nil {
		t.Errorf("Send = %v, want SES to take the mail from a verified domain", err)
	}
}

// SES refuses a sender that no verified identity covers: the refusal that a
// sender identity which is missing, or not verified yet, causes in AWS.
func TestSendFailsWhenSESRefusesTheSender(t *testing.T) {
	client := sesv2.NewFromConfig(awstest.Config(t))
	mailer, err := ses.New(ses.Config{Client: client, From: "Meeting rooms <rooms@" + domain() + ">"})
	if err != nil {
		t.Fatal(err)
	}
	err = mailer.Send(t.Context(), message())
	var rejected *types.MessageRejected
	if !errors.As(err, &rejected) {
		t.Errorf("Send = %v, want the refusal MessageRejected of SES", err)
	}
}
