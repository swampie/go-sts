package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/chromedp"
	credentials_manager "github.com/swampie/go-to-sts/cmd/go-to-sts/credentials"
	"github.com/swampie/go-to-sts/cmd/go-to-sts/parser"
)

type StsConfig struct {
	spId               int
	idpId              string
	awsProfile         string
	awsRoleArn         string
	sessionDuration    int
	email              string
	password           string
	awsCredentialsFile string
}

func (c StsConfig) BuildUrl() string {
	return fmt.Sprintf("https://accounts.google.com/o/saml2/initsso?idpid=%s&spid=%d&forceauthn=false", c.idpId, c.spId)
}

func (c StsConfig) Validate() bool {
	var isValid = true
	if c.spId <= 0 {
		log.Fatal("--sp-id argument should be a valid integer")
		isValid = false
	}
	if c.idpId == "" {
		log.Fatal("--idp-id argument should be a valid integer")
		isValid = false
	}
	if c.awsRoleArn == "" {
		log.Fatal("--aws-role-arn argument should be a valid arn format")
		isValid = false
	}

	if c.email == "" {
		log.Fatal("--email should be a valid non empty email format")
		isValid = false
	}

	if c.password == "" {
		log.Fatal("--password should be a valid non empty string")
		isValid = false
	}

	return isValid
}

func main() {
	config := BuildConfig()
	if !config.Validate() {
		os.Exit(1)
	}
	log.Println("Hello go-to-sts")
	Start(config)

}

func BuildConfig() StsConfig {
	var spId int
	var idpId string
	var awsProfile string
	var awsRoleArn string
	var sessionDuration int
	var email string
	var password string
	var awsCredentialsFile string
	flag.IntVar(&spId, "sp-id", 0, "--sp-id")
	flag.StringVar(&idpId, "idp-id", "", "--idp-id")
	flag.StringVar(&awsProfile, "aws-profile", "", "--aws-profile")
	flag.StringVar(&awsRoleArn, "aws-role-arn", "", "--aws-role-arn")
	flag.IntVar(&sessionDuration, "session-duration", 5*60, "--session-duration")
	flag.StringVar(&email, "email", "", "--email")
	flag.StringVar(&password, "password", "", "--password")
	dir, _ := os.UserHomeDir()
	flag.StringVar(&awsCredentialsFile, "aws-credentials-file", fmt.Sprintf("%s/.aws/credentials", dir), "aws credentials file")
	flag.Parse()

	return StsConfig{spId: spId,
		idpId:              idpId,
		awsProfile:         awsProfile,
		awsRoleArn:         awsRoleArn,
		sessionDuration:    sessionDuration,
		email:              email,
		password:           password,
		awsCredentialsFile: awsCredentialsFile,
	}

}

func Start(c StsConfig) {
	var opts [](chromedp.ExecAllocatorOption) = append(chromedp.DefaultExecAllocatorOptions[:], chromedp.Flag("headless", false))
	opts = append(opts)

	homeDir, dirErr := os.UserHomeDir()
	if dirErr != nil {
		log.Printf("Unable to get user directory. Browser session will not be persisted")
	} else {
		opts = append(opts, chromedp.UserDataDir(fmt.Sprintf("%s/.config/go-sts", homeDir)))
	}
	actx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	var contextOpts = []chromedp.ContextOption{}
	contextOpts = []chromedp.ContextOption{
		chromedp.WithLogf(log.Printf),
		//chromedp.WithDebugf(log.Printf),
		chromedp.WithErrorf(log.Printf),
	}
	ctx, cancel := chromedp.NewContext(actx, contextOpts...)

	// create a timeout
	ctx, cancel = context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()

	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *fetch.EventRequestPaused:
			go func() {
				c := chromedp.FromContext(ctx)
				e := cdp.WithExecutor(ctx, c.Target)
				req := fetch.ContinueRequest(ev.RequestID)
				if err := req.Do(e); err != nil {
					log.Printf("Failed to continue request: %v", err)
				}
			}()
		}
	})

	// navigate to a page, wait for an element, click
	log.Printf("Navigating to %s", c.BuildUrl())

	var submitFirst []*cdp.Node
	var samlTarget string
	var SAMLResponse string
	err := chromedp.Run(ctx,

		chromedp.Navigate(c.BuildUrl()),
		// wait for footer element is visible (ie, page is loaded)
		chromedp.ActionFunc(func(ctx context.Context) error {
			var str string
			chromedp.Location(&str).Do(ctx)

			if str == "https://signin.aws.amazon.com/saml" {
				a := chromedp.Value(`input[name=SAMLResponse]`, &SAMLResponse, chromedp.ByQuery)
				a.Do(ctx)
				_ = chromedp.Cancel(ctx)
				return nil
			}
			return nil
		}),

		chromedp.WaitVisible(`input#identifierId`),
		// set email
		chromedp.SetValue(`#identifierId`, c.email, chromedp.ByID),
		// find and click "Example" link
		chromedp.Nodes("button", &submitFirst, chromedp.ByQueryAll),

		chromedp.ActionFunc(func(ctx context.Context) error {
			chromedp.MouseClickNode(submitFirst[2]).Do(ctx)
			return nil
		}),
		chromedp.Sleep(3*time.Second),
		// retrieve the text of the textarea
		chromedp.WaitVisible(`input[name=password]`),
		// fill the password
		chromedp.SetValue(`input[name=password]`, c.password, chromedp.ByQuery),
		// submit
		chromedp.Click("#passwordNext div button span"),

		chromedp.WaitVisible(`input[type=radio]`),
		chromedp.Location(&samlTarget),
		fetch.Enable(),
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Printf("hello %s", samlTarget)
			return nil
		}),

		chromedp.WaitNotVisible(`input[name=SAMLResponse]`),
		chromedp.Value(`input[name=SAMLResponse]`, &SAMLResponse, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
		chromedp.ActionFunc(func(ctx context.Context) error {
			_ = chromedp.Cancel(ctx)
			return nil
		}),
	)

	if err != nil {
		log.Print(err)
	}

	if SAMLResponse != "" {
		parser := parser.New()
		cm := credentials_manager.New(parser)
		r := cm.PrepareRoleWithSAML(SAMLResponse, c.awsRoleArn)
		cm.AssumeRoleWithSAML(SAMLResponse, c.awsCredentialsFile, c.awsProfile, r, c.sessionDuration)
		log.Printf("Everything good")
	}
}

func fullScreenshot(urlstr string, quality int, res *[]byte) chromedp.Tasks {
	return chromedp.Tasks{
		chromedp.Navigate(urlstr),
		chromedp.FullScreenshot(res, quality),
	}
}
