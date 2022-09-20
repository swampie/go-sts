package credentials_manager

import (
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/swampie/go-to-sts/cmd/go-to-sts/parser"
	ini "gopkg.in/ini.v1"
)

type credentials_manager struct {
	parser parser.Parser
}

type Credentials struct {
	accessKeyId     string
	expiration      string
	secretAccessKey string
	sessionToken    string
	awsRoleArn      string
}

func New(p parser.Parser) credentials_manager {
	c := credentials_manager{parser: p}
	return c
}

func (c credentials_manager) PrepareRoleWithSAML(samlResponse string, customArn string) parser.Role {
	roles := c.parser.ParseSamlResponse(samlResponse, customArn)
	idx := sliceIndex(len(roles), func(i int) bool { return roles[i].RoleArn == customArn })
	if idx == -1 {
		errorString := fmt.Sprintf("Unable to find role %s in available roles", customArn)
		panic(errorString)
	}
	return roles[idx]

}

func (c credentials_manager) AssumeRoleWithSAML(samlAssertion string, awsCredentialsFile string, awsProfile string, role parser.Role, sessionDuration int) {
	mySession := session.Must(session.NewSession())

	// Create a STS client from just a session.
	svc := sts.New(mySession)
	input := &sts.AssumeRoleWithSAMLInput{
		DurationSeconds: aws.Int64(int64(sessionDuration)),
		PrincipalArn:    aws.String(role.PrincipalArn),
		RoleArn:         aws.String(role.RoleArn),
		SAMLAssertion:   aws.String(samlAssertion),
	}
	result, err := svc.AssumeRoleWithSAML(input)
	if err == nil {
		log.Printf("Response is %s", result.Credentials)
		exp := result.Credentials.Expiration.Format("2022-09-12T11:22:27.000Z")
		c.saveCredentials(awsCredentialsFile, awsProfile, Credentials{
			accessKeyId:     *result.Credentials.AccessKeyId,
			secretAccessKey: *result.Credentials.SecretAccessKey,
			expiration:      exp,
			awsRoleArn:      role.RoleArn,
			sessionToken:    *result.Credentials.SessionToken,
		})
	} else {
		log.Fatalf("An error occurred %s", err)
	}

}

func (c credentials_manager) saveCredentials(path string, profile string, credentials Credentials) {
	log.Printf("Saving credentials on %s for profile %s", path, profile)
	cfg, err := ini.Load(path)
	if err != nil {
		log.Printf("Unable to parse credentials from file %s", path)
	}
	cfg.Section(profile).Key("aws_access_key_id").SetValue(credentials.accessKeyId)
	cfg.Section(profile).Key("aws_session_expiration").SetValue(credentials.expiration)
	cfg.Section(profile).Key("aws_secret_access_key").SetValue(credentials.secretAccessKey)
	cfg.Section(profile).Key("aws_role_arn").SetValue(credentials.awsRoleArn)
	cfg.Section(profile).Key("aws_session_token").SetValue(credentials.sessionToken)

	cfg.SaveTo(path)
}

func (c credentials_manager) loadCredentials(path string, profile string) *Credentials {
	_ = ini.MapToWithMapper(&Credentials{}, ini.TitleUnderscore, []byte("package_name=ini"))
	cfg, err := ini.Load(path)
	if err != nil {
		log.Printf("Unable to parse credentials from file %s", path)
		return nil
	}
	p := new(Credentials)
	err = cfg.Section(profile).MapTo(p)
	if err != nil {
		log.Fatalf("Unable to parse credentials file content %s", err)
	}
	return p

}

func (c credentials_manager) SessionExpirationFromCredentials(awsCredentialsFile string, profile string, roleArn string) (bool, int64) {

	p := c.loadCredentials(awsCredentialsFile, profile)
	if p != nil {
		if roleArn != "" && p.awsRoleArn != roleArn {
			log.Printf("Found credentials for a different role ARN (found \"%s\" != received \"%s\")", p.awsRoleArn, roleArn)
			return false, -1
		}

		parsedTime, err := time.Parse("2022-09-12T11:22:27.000Z", p.expiration)
		if err != nil {
			log.Fatalf("Unable to parse Expiration in %s. Please check file", awsCredentialsFile)
		}
		if (parsedTime.Add(-time.Millisecond * (30 * 1000))).After(time.Now()) {
			return true, parsedTime.Add(-time.Millisecond * (30 * 1000)).UTC().Unix()
		}
		return false, parsedTime.UTC().Unix()

	} else {
		return false, -1
	}
}

func sliceIndex(limit int, predicate func(i int) bool) int {
	for i := 0; i < limit; i++ {
		if predicate(i) {
			return i
		}
	}
	return -1
}
