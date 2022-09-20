package parser

import (
	"log"
	"regexp"
	"strconv"

	"github.com/RobotsAndPencils/go-saml"
)

type Parser struct {
}

type Role struct {
	RoleArn         string
	roleName        string
	samlProvider    string
	PrincipalArn    string
	sessionDuration int
}

func New() Parser {
	p := Parser{}

	return p
}

func (p Parser) ParseSamlResponse(response string, roleToAssume string) []Role {
	rolePattern := regexp.MustCompile("(arn:(aws|aws-us-gov|aws-cn):iam:[^:]*:[0-9]+:role/([^,]+))")
	principalPattern := regexp.MustCompile("(arn:aws:iam:[^:]*:[0-9]+:saml-provider/[^,]+)")

	samlResponse, err := saml.ParseEncodedResponse(response)
	if err != nil {
		log.Fatalf("unable to parse SAML Response %s: %s", response, err.Error())
		return nil
	}

	log.Printf("%s", samlResponse)
	var sessionDuration int
	sessionDuration, err = strconv.Atoi(samlResponse.GetAttribute("https://aws.amazon.com/SAML/Attributes/SessionDuration"))
	if err != nil {
		log.Printf("Unable to parse session Duration %s. Setting it to 4 hours", err.Error())
		sessionDuration = 60 * 60 * 4
	}

	roles := samlResponse.GetAttributeValues("https://aws.amazon.com/SAML/Attributes/Role")
	result := []Role{}
	for _, role := range roles {
		principalMatches := principalPattern.MatchString(role)
		roleMatches := rolePattern.MatchString(role)
		if !principalMatches || !roleMatches {
			continue
		}
		principalMatcher := principalPattern.FindStringSubmatch(role)
		roleMatcher := rolePattern.FindStringSubmatch(role)
		roleArn := roleMatcher[1]
		roleName := roleMatcher[3]
		samlProvider := principalMatcher[1]
		result = append(result, Role{RoleArn: roleArn, roleName: roleName, samlProvider: samlProvider, sessionDuration: sessionDuration, PrincipalArn: principalMatcher[1]})

	}
	return result

}
