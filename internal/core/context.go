package core

import "fmt"

// CloudContext represents a single authentication/region scope
// for a cloud provider (e.g. AWS profile + region, or a GCP project).
type CloudContext struct {
	Provider          string
	ContextName       string
	AccountID         string
	AccountName       string
	Region            string
	Tenant            string
	CredentialProfile string
	AuthMode          string
	CredentialScope   string
}

func (c CloudContext) DisplayName() string {
	if c.ContextName != "" {
		return c.ContextName
	}
	if c.Provider == "AWS" {
		switch {
		case c.AccountID != "" && c.AccountName != "" && c.AccountID != c.AccountName:
			return fmt.Sprintf("%s (%s)", c.AccountID, c.AccountName)
		case c.AccountID != "":
			return c.AccountID
		}
	}
	if c.AccountName != "" {
		return c.AccountName
	}
	return c.AccountID
}

func (c CloudContext) AuthRef() string {
	if c.ContextName != "" {
		return c.ContextName
	}
	if c.CredentialProfile != "" {
		return c.CredentialProfile
	}
	if c.AccountName != "" {
		return c.AccountName
	}
	return c.AccountID
}

func (c CloudContext) CacheKey() string {
	return fmt.Sprintf("%s|%s|%s|%s", c.Provider, c.AccountID, c.Region, c.AuthRef())
}
